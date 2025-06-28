#!/usr/bin/env bash

# watch code changes, trigger re-build, and kill process 
while true; do
   go build -o build/genie ./cmd && pkill -f 'build/genie'
   # inotifywait -e attrib $(find . -name '*.go') || exit
   fswatch -o ./ | xargs -n1 -I{} ./build/genie --tui gocui
done
