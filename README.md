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
  -H value
        Add custom http headers
  -c string
        Specify cookie Header value
  -j string
        Specifies the jump host
  -k string
        Specifies the jump host secret
  -n int
        Split into N segments and download in parallel (default 10)
  -o string
        Specify download output file (default is auto detect)
  -q    Specifies whether it should be quiet
  -r string
        Specify referrer
  -ua string
        Specify User Agent (default "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36")
```
# What is a jump host?
Jump host is a tool I wrote (similar to proxy, but light weight, secure by default).
Checkout https://github.com/wushilin/netjumper

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

# with custom header
$ pwget -H "someHeader: headerval" -o small.iso http://some-host.download.com/super-large.iso

# use with some jumper
$ pwget -j "jumpHost:9527" -k "secret" -H "someHeader: headerval" -o small.iso http://some-host.download.com/super-large.iso

Note that if jumper can't be connected, it will fall back to normal dialer (e.g. direct connection)

# use with quiet (no progress reporting)
$ pwget -q -H "someHeader: headerval" -o small.iso http://some-host.download.com/super-large.iso

```

Sample output

```
root@master /o/G/s/g/w/pwget# ./pwget -j home.myhome.net:9527 -k bigsecret "http://releases.ubuntu.com/18.04.1/ubuntu-18.04.1-desktop-amd64.iso?_ga=2.191772394.1990563598.1542159661-244726927.1539156732"
Quiet? false
Size 1953349632 Bytes, file name ubuntu-18.04.1-desktop-amd64.iso
Each segment 195334963 Bytes
Progress: 168057KB of 1907568KB (8%)
```
## Resume capable
The program aborts only after 10 consecutive retries (with 5 seconds sleep inbetween retries) failed to download a single byte.
The program resumes download from last abortion (based on file part size), however if you used -n to specify threads, the n must be the same
otherwise this download may create corrupted file

Note: If server doesn't support Content-Length response header, the program will not download.

Enjoy!

