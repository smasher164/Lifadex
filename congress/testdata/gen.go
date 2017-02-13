package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sync"
)

func dumpresponse(rawurl string) error {
	resp, err := http.Get(rawurl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	filename := path.Base(path.Dir(rawurl)) + "_" + path.Base(rawurl) + ".html"
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	io.Copy(f, resp.Body)
	f.Sync()
	return nil
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	var wg sync.WaitGroup
	for scanner.Scan() {
		s := scanner.Text()
		wg.Add(1)
		go func(str string) {
			if err := dumpresponse(s); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			wg.Done()
		}(s)
	}
	if err := scanner.Err(); err != nil {
		fmt.Println(err)
	}
	wg.Wait()
}
