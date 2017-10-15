package main

import (
	"fmt"
	"flag"
	"os"
	"net/http"
	"strings"
	"sync"
	"io"
	"time"
	"net/url"
	"sync/atomic"
)

var nsegs = flag.Int64("n", 10, "Split into N segments and download in parallel");

var output = flag.String("o", "", "Specify download output file (default is auto detect)");

var cookie = flag.String("c", "", "Specify cookie Header value (default is no cookie)")

const DEFAULT_UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36"

var ref = flag.String("r", "", "Specify referrer (default is root of downloading host)")

var ua = flag.String("ua", DEFAULT_UA, "Specify User Agent")

func main() {
	flag.Parse()
	fmt.Println("Nsegs = ", *nsegs, "output = ", *output)
	remainingArgs := flag.Args()

	if len(remainingArgs) > 1 || len(remainingArgs) == 0 {
		flag.PrintDefaults()
		fmt.Println("Need one and only one url.");
		os.Exit(1);
	}

	urlArg := remainingArgs[0];
	newUrl, cl, fn, err := probe(urlArg, *cookie)

	var urlReal *url.URL
	if newUrl != nil {
		urlReal = newUrl
	} else {
		urlReal,err = url.Parse(urlArg)
		if err != nil {
			panic(err)
		}
	}
	if err != nil {
		panic(err)
	}

	if *output != "" {
		fn = *output
	}

	if stat, err := os.Stat(fn); err == nil {
		fmt.Println("File", fn, "already exists, size is", stat.Size(), "bytes. Please delete first.")
		os.Exit(1)
	}
	fmt.Println("Size", cl, "Bytes, file name", fn)
	if cl < 10240 {
		*nsegs = 1
	}

	wg := new(sync.WaitGroup)

	segSize := cl / *nsegs;
	fmt.Println("Each segment", segSize, "Bytes")
	filenames := make([]string, int(*nsegs))

	var downloaded uint64
	downloaded = 0
	for i := 0; i < int(*nsegs); i++ {
		wg.Add(1)
		segStart := int64(i) * segSize
		segEnd := int64(i)*segSize + segSize - 1
		if segEnd > cl-1 || int64(i) == *nsegs-1 {
			segEnd = cl - 1
		}
		next_fn := fmt.Sprintf("%s_part_%04d", fn, i)
		go downloadPart(urlReal, *cookie, next_fn, i, segStart, segEnd, cl, fn, wg, &downloaded);
		filenames[i] = next_fn
	}

	go func() {
		modCount := 0
		for {
			newModCount := (int)(downloaded/1024/1024)
			if newModCount > modCount {
				modCount = newModCount
				fmt.Println("Downloaded", modCount, "MB so far")
			}
			time.Sleep(500*time.Millisecond)
		}
	}()
	wg.Wait()

	final_out, err := os.Create(fn)
	if err != nil {
		panic(err)
	}
	defer final_out.Close()
	for _, next_fn := range (filenames) {
		tmp, err := os.Open(next_fn)
		if err != nil {
			panic(err)
		}
		io.Copy(final_out, tmp)
		tmp.Close()
		os.Remove(next_fn)
	}
	fmt.Println("Done")
}

func downloadPart(urlR *url.URL,cookie, filename string, i int, segStart,
	segEnd, total int64, fn string, wg *sync.WaitGroup, downloaded *uint64) {
	defer wg.Done()
	errorCount := 0
	for {
		copied, err := downloadPart1(urlR, cookie, filename, i, segStart, segEnd, total, fn, wg, downloaded);
		if err == nil || err == io.EOF {
			return
		} else if copied > 0 {
			errorCount = 0
		} else {
			errorCount++
		}
		if errorCount > 10 {
			panic(err)
		}
		fmt.Println("Error occured - will retry", err)
		time.Sleep(5*time.Second)
	}
}

func downloadPart1(urlR *url.URL, cookie, filename string, i int, segStart, segEnd int64, total int64,
	fn string, wg *sync.WaitGroup, downloaded *uint64) (int64, error){
	var additionalOffset int64 = 0
	if stat, err := os.Stat(filename); err == nil {
		additionalOffset += stat.Size()
	}
	var out *os.File
	var err error
	if additionalOffset == 0 {
		out, err = os.Create(filename)
	} else {
		out, err = os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0600)
	}
	if err != nil {
		return 0, err
	}

	defer out.Close()
	client := http.Client{}

	req, err := http.NewRequest("GET", urlR.String(), nil)

	if err != nil {
		return 0, err
	}
	//Content-Range: <unit> <range-start>-<range-end>/<size>
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", segStart+additionalOffset, segEnd))
	req.Header.Add("User-Agent", *ua);
	req.Header.Add("Referer", referrer(urlR.String()))
	if cookie != "" {
		req.Header.Add("Cookie", cookie)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	buf := make([]byte, 32*1024)
	var copied int64 = 0
	err = nil
	for {
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := out.Write(buf[0:nr])
			if nw > 0 {
				atomic.AddUint64(downloaded, uint64(nw))
				copied += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return copied, err
}

func probe(urlReal, cookie string) (*url.URL, int64, string, error) {
	request, err := http.NewRequest("GET", urlReal, nil)

	if err != nil {
		return nil, 0, "", err;
	}

	request.Header.Add("User-Agent", *ua)
	request.Header.Add("Referer", referrer(urlReal))
	if cookie != "" {
		request.Header.Add("Cookie", cookie)
	}
	client := http.Client{}

	resp, err := client.Do(request)

	if err != nil {
		return nil, 0, "", err
	}

	defer resp.Body.Close()
	var cl = resp.ContentLength

	fmt.Println(resp.Header)

	var fn string
	cd := resp.Header.Get("Content-Disposition")
	location, err := resp.Location()
	if err != nil {
		location = nil
	}

	cd = strings.ToLower(cd)
	if len(cd) > 0 && strings.Index(cd, "filename=") != -1 {
		fn = cd[strings.Index(cd, "filename=")+9:]
		for strings.Index(fn, "\"") == 0 {
			fn = fn[1:]
		}

		for strings.LastIndex(fn, "\"") == len(fn)-1 {
			fn = fn[:len(fn)-1]
		}
	} else {
		fn = urlReal[strings.LastIndex(urlReal, "/")+1:]
	}

	if fn == "" {
		fn = "DOWNLOAD_NO_NAME"
	}
	return location, cl, fn, nil
}

func referrer(origUrl string) string {
	if *ref != "" {
		return *ref;
	}
	doubleSlash := strings.Index(origUrl, "//")
	if doubleSlash == -1 {
		return origUrl
	}

	lastSlash := strings.LastIndex(origUrl[doubleSlash + 2:], "/")
	if(lastSlash == -1) {
		return origUrl
	}

	return origUrl[:lastSlash];
}