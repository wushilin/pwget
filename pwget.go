package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var nsegs = flag.Int64("n", 10, "Split into N segments and download in parallel")

var jumpHost = flag.String("j", "", "Specifies the jump host")
var jumpHostSecret = flag.String("k", "", "Specifies the jump host secret")
var output = flag.String("o", "", "Specify download output file (default is auto detect)")

var cookie = flag.String("c", "", "Specify cookie Header value")

const DEFAULT_UA = "curl/7.64.1"

var ref = flag.String("r", "", "Specify referrer")
var clstring = flag.String("l", "", "Specify a content length")

var clint int64 = -1

var ua = flag.String("ua", DEFAULT_UA, "Specify User Agent")

var quiet = flag.Bool("q", false, "Specifies whether it should be quiet")

type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join([]string(*i), ",")
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var headers arrayFlags

func parseHeader(input string) (string, string) {
	idx := strings.Index(input, ":")
	if idx < 0 {
		return "", ""
	}
	name := input[:idx]
	val := input[idx+1:]

	name = strings.TrimSpace(name)
	val = strings.TrimSpace(val)
	return name, val
}

func main() {
	flag.Var(&headers, "H", "Add custom http headers")
	flag.Parse()
	var err error = nil
	if *clstring != "" {
		clint, err = strconv.ParseInt(*clstring, 10, 64)
		if err != nil {
			panic(err)
		}
	}
	remainingArgs := flag.Args()

	if len(remainingArgs) > 1 || len(remainingArgs) == 0 {
		flag.PrintDefaults()
		fmt.Println("Need one and only one url.")
		os.Exit(1)
	}

	urlArg := remainingArgs[0]
	newUrl, cl, fn, err := probe(urlArg, *cookie)

	var urlReal *url.URL
	if newUrl != nil {
		urlReal = newUrl
	} else {
		urlReal, err = url.Parse(urlArg)
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
	fmt.Println("Quiet?", *quiet)
	fmt.Println("Size", cl, "Bytes, file name", fn)
	if cl < 0 {
		fmt.Println("No content length info, skipped. If you know what is the file size in bytes, please specify using -l 12345 (where 12345 is the file size)")
		os.Exit(1)
	}
	if cl < 10240 {
		*nsegs = 1
	}
	wg := new(sync.WaitGroup)

	segSize := cl / *nsegs
	fmt.Println("Each segment", segSize, "Bytes")
	filenames := make([]string, int(*nsegs))

	var downloaded int64
	downloaded = 0
	for i := 0; i < int(*nsegs); i++ {
		wg.Add(1)
		segStart := int64(i) * segSize
		segEnd := int64(i)*segSize + segSize - 1
		if segEnd > cl-1 || int64(i) == *nsegs-1 {
			segEnd = cl - 1
		}
		next_fn := fmt.Sprintf("%s_part_%04d", fn, i)
		go downloadPart(urlReal, *cookie, next_fn, i, segStart, segEnd, cl, fn, wg, &downloaded)
		filenames[i] = next_fn
	}

	if !*quiet {
		wg.Add(1)
		go func() {
			defer wg.Done()
			modCount := 0
			totalKb := (int)(cl / 1024)
			for {
				newModCount := (int)(downloaded / 1024)
				percent := "-"
				if cl != 0 {
					percentNumber := (int)(downloaded * int64(100) / cl)
					percent = fmt.Sprintf("%d", percentNumber)
				}
				if newModCount > modCount || int64(downloaded) == cl {
					modCount = newModCount
					fmt.Printf("\rProgress: %dKB of %dKB (%s%%)", modCount, totalKb, percent)
				}
				if int64(downloaded) == cl {
					fmt.Println("")
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
		}()
	}
	wg.Wait()

	fmt.Println("Merging into", fn)
	final_out, err := os.Create(fn)
	if err != nil {
		panic(err)
	}
	defer final_out.Close()
	for _, next_fn := range filenames {
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

func downloadPart(urlR *url.URL, cookie, filename string, i int, segStart,
	segEnd, total int64, fn string, wg *sync.WaitGroup, downloaded *int64) {
	defer wg.Done()
	errorCount := 0
	tryNumber := 0
	for {
		copied, err := downloadPart1(urlR, cookie, filename, i, segStart, segEnd, total, fn, wg, downloaded, tryNumber)
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
		if segEnd < 0 {
			fmt.Println("Retry not supported, aborting...")
			os.Remove(filename)
			panic(err)
		}
		time.Sleep(5 * time.Second)
		tryNumber++
	}
}

func makeClient() *http.Client {
	return makeClientOld()
}

func makeClientOld() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	return client
}
func downloadPart1(urlR *url.URL, cookie, filename string, i int, segStart, segEnd int64, total int64,
	fn string, wg *sync.WaitGroup, downloaded *int64, tryNumber int) (int64, error) {
	var additionalOffset int64 = 0
	if stat, err := os.Stat(filename); err == nil {
		additionalOffset += stat.Size()
	}
	var out *os.File
	var err error
	if additionalOffset == 0 {
		out, err = os.Create(filename)
	} else {
		if tryNumber == 0 {
			atomic.AddInt64(downloaded, int64(additionalOffset))
		}
		out, err = os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0600)
	}
	if err != nil {
		return 0, err
	}

	defer out.Close()
	client := makeClient()

	req, err := http.NewRequest("GET", urlR.String(), nil)

	if err != nil {
		return 0, err
	}
	var expectedLength int64 = -1
	if segEnd > 0 {
		expectedLength = segEnd - (segStart + additionalOffset)
	}
	if expectedLength <= 0 {
		return 0, nil
	}
	//Content-Range: <unit> <range-start>-<range-end>/<size>
	if segEnd > 0 {
		// content length is known!
		req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", segStart+additionalOffset, segEnd))
	}
	for _, hdr := range headers {
		k, v := parseHeader(hdr)
		if k != "" && v != "" {
			req.Header.Add(k, v)
		} else {
			// the header is ignored
		}
	}

	req.Header.Add("User-Agent", *ua)
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
				atomic.AddInt64(downloaded, int64(nw))
				copied += int64(nw)
				if copied >= expectedLength {
					return copied, nil
				}
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
		return nil, 0, "", err
	}

	request.Header.Add("User-Agent", *ua)
	request.Header.Add("Referer", referrer(urlReal))
	if cookie != "" {
		request.Header.Add("Cookie", cookie)
	}
	for _, hdr := range headers {
		k, v := parseHeader(hdr)
		if k != "" && v != "" {
			request.Header.Add(k, v)
		} else {
			// the header is ignored
		}
	}
	client := makeClient()

	resp, err := client.Do(request)

	if err != nil {
		return nil, 0, "", err
	}

	defer resp.Body.Close()
	var clstr = resp.Header.Get("Content-Length")
	fmt.Println("CL", clstr, resp.Header)
	cl, err := strconv.ParseInt(clstr, 10, 64)
	if err != nil || cl == 0 {
		cl = -1
	}
	if clint != -1 {
		cl = clint
	}
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
	if strings.Index(fn, "?") != -1 {
		fn = fn[:strings.Index(fn, "?")]
	}
	return location, cl, fn, nil
}

func referrer(origUrl string) string {
	if *ref != "" {
		return *ref
	}
	doubleSlash := strings.Index(origUrl, "//")
	if doubleSlash == -1 {
		return origUrl
	}

	lastSlash := strings.LastIndex(origUrl[doubleSlash+2:], "/")
	if lastSlash == -1 {
		return origUrl
	}

	return origUrl[:lastSlash]
}
