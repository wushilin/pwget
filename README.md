# pwget
Similar to wget, but in parallel

Ever downloaded large files over internet, but connection often resets?
Ever want to download in parallel using simple command like wget?

You have it now.

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

