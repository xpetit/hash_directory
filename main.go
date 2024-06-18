package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/xpetit/x/v4"
	"lukechampine.com/blake3"
)

func main() {
	workers := flag.Int("w", runtime.NumCPU(), "Number of disk workers")
	flag.Parse()
	Assert(flag.NArg() == 1, "expected argument: DIRECTORY")

	t := time.Now()
	var m sync.Mutex
	hashByPath := map[string][]byte{}
	var n atomic.Int64
	{
		pathsC := make(chan string, 1000)
		wait := Goroutines(*workers, func(int) {
			for path := range pathsC {
				b3 := blake3.New(32, nil)
				n.Add(Must(io.Copy(b3, CloseAfterRead(Must(os.Open(path))))))
				h := b3.Sum(nil)
				m.Lock()
				hashByPath[path] = h
				m.Unlock()
			}
		})
		Check(filepath.WalkDir(flag.Arg(0), func(path string, d fs.DirEntry, err error) error {
			Check(err)
			if d.Type().IsRegular() {
				pathsC <- path
			}
			return nil
		}))
		close(pathsC)
		wait()
	}
	b3 := blake3.New(32, nil)
	for _, path := range Sort(Keys(hashByPath)) {
		b3.Write(hashByPath[path])
	}
	result := hex.EncodeToString(b3.Sum(nil))
	secs := time.Since(t).Seconds()
	fmt.Println(result)
	fmt.Fprintf(os.Stderr, "hash rate: %.f MB/s\n", float64(n.Load())/secs/1e6)
}
