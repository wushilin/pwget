# pwget
Similar to wget, but in parallel

Ever downloaded large files over internet, but connection often resets?
Ever want to download in parallel using simple command like wget?

You have it now.

## Installing...
```bash
$ go get -u github.com/wushilin/pwget
```

After this, you will find pwget executable in your $GOPATH/bin. You can add this to your $PATH, or invoke with full path.

## Usage
```bash
$ pwget <url>
```

Full usage explanation
```bash
root@paladin ~# pwget
  -c string
        Specify cookie Header value
  -n int
        Split into N segments and download in parallel (default 10)
  -o string
        Specify download output file (default is auto detect)
  -r string
        Specify referrer
  -ua string
        Specify User Agent (default "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36")
```

# Features of pwget

## Simple
```bash
# Download with 10 slices
$ pwget http://some-host.download.com/super-large.iso

# Download with 50 slices
$ pwget -n 50 http://some-host.download.com/super-large.iso

# With cookie
$ pwget -c "abc=def; other=sucks;" http://some-host.download.com/super-large.iso

# with referer
$ pwget -r "http://some-host.download.com" http://some-host.download.com/super-large.iso

# with User-Agent
$ pwget -ua "Some-Weird-Broser-String" http://some-host.download.com/super-large.iso

# Specify output file
$ pwget -o small.iso http://some-host.download.com/super-large.iso
```

## Resume capable
The program aborts only after 10 consecutive retries (with 5 seconds sleep inbetween retries) failed to download a single byte.
The program resumes download from last abortion (based on file part size)

Note: If server doesn't support Content-Length response header, the program will not download in split slices.

Enjoy!
