package congress

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	errMod = errors.New("Last-Modified is an older value")
	errHdr = errors.New(`First header is not "Last-Modified"`)
)

var (
	base = mustparse("https://www.gpo.gov/fdsys/")
	cwd  = muststring(os.Getwd())
)

var govbulkClient = &http.Client{
	Transport: http.RoundTripper(&http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    true,
	}),
}

func mustparse(rawurl string) *url.URL {
	u, err := url.Parse(rawurl)
	if err != nil {
		panic(err)
	}
	return u
}

func muststring(s string, err error) string {
	if err != nil {
		panic(err)
	}
	return s
}

type semaphore chan struct{}

func newsemaphore(n int) semaphore {
	s := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		s <- struct{}{}
	}
	return s
}

func (s semaphore) acquire() {
	<-s
}

func (s semaphore) release() {
	s <- struct{}{}
}

type dwndata struct {
	fileurl *url.URL
	ext     string
}

type crawler struct {
	sources  []dwndata
	progress uint64
	wg       sync.WaitGroup
	semaphore
}

func newcrawler(climit int) *crawler {
	return &crawler{
		[]dwndata{},
		0,
		sync.WaitGroup{},
		newsemaphore(climit),
	}
}

// update progress meter on stdout
func (c *crawler) update() {
	atomic.AddUint64(&c.progress, 1)
	fmt.Printf("\rProgress: %d/%d", c.progress, len(c.sources))
}

// download fetches the downloaded file, and stores it in
// its respective directory in a tarball, consisting of
// the Last-Modified header (for caching purposes), and
// the file itself.
func (c *crawler) download(fileurl *url.URL, ext string) {
	c.wg.Add(1)
	defer c.wg.Done()

	defer c.update()

	c.acquire()
	defer c.release()

	dir, fileName := filepath.Split(fileurl.Path)
	// absolute path
	dir = filepath.Join(cwd, dir)
	tarName := fileName[:len(fileName)-len(ext)] + ".tar"

	// retrieve the resource
	resp, err := govbulkClient.Get(fileurl.String())
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	// check the "Last-Modified" header to see if it has changed
	lastmod := []byte(resp.Header.Get("Last-Modified"))
	switch err := c.checkMod(filepath.Join(dir, tarName), lastmod); {
	case os.IsNotExist(err), err == errMod:
		// proceed
		// fmt.Println("Miss", fileName)
	case err == nil:
		// exit function
		// fmt.Println("Hit", fileName)
		return
	default:
		log.Fatalln(err)
	}

	// Create file and any parent directories along the way
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Fatalln(err)
	}
	tarFile, err := os.Create(filepath.Join(dir, tarName))
	if err != nil {
		log.Fatalln(err)
	}
	defer tarFile.Close()

	// create a tar writer to write to file
	tw := tar.NewWriter(tarFile)
	defer tw.Close()

	// Write header for for last-modified header
	lmHdr := &tar.Header{
		Name: "Last-Modified.txt",
		Mode: int64(os.ModePerm),
		Size: int64(len(lastmod)),
	}
	if err := tw.WriteHeader(lmHdr); err != nil {
		log.Fatalln(err)
	}
	// Write body for the last-modified header
	if _, err := tw.Write(lastmod); err != nil {
		log.Fatalln(err)
	}

	// Write header for downloaded file
	fHdr := &tar.Header{
		Name: fileName,
		Mode: int64(os.ModePerm),
		Size: resp.ContentLength,
	}
	if err := tw.WriteHeader(fHdr); err != nil {
		log.Fatalln(err)
	}

	// Stream response body for downloaded file
	if _, err := io.Copy(tw, resp.Body); err != nil {
		log.Fatalln(err)
	}
}

// checkMod checks the Last-Modified header in the tarball against
// a byte slice and returns an error, nil if they are equal.
func (c *crawler) checkMod(tarPath string, lastmod []byte) error {
	tarFile, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer tarFile.Close()
	tr := tar.NewReader(tarFile)
	hdr, err := tr.Next()
	if err != nil {
		return err
	} else if hdr.Name != "Last-Modified.txt" {
		return errHdr
	}
	b := make([]byte, hdr.Size)
	if _, err := tr.Read(b); err != nil {
		return err
	}
	// Check for newer last-modified date
	// instead of difference?
	if !bytes.Equal(lastmod, b) {
		return errMod
	}
	return nil // equal
}

func (c *crawler) crawl(rawurl string) {
	c.wg.Add(1)
	defer c.wg.Done()

	c.acquire()
	doc, err := goquery.NewDocument(rawurl)
	if err != nil {
		log.Fatalln(err)
	}
	c.release()
	doc.Find("#bulkdata td").Each(func(i int, s *goquery.Selection) {
		if s.Text() == "Parent Directory" {
			return
		}
		rel, exists := s.ChildrenFiltered("a").Attr("href")
		if !exists {
			return
		}
		abs := base.ResolveReference(mustparse(rel))
		ext := path.Ext(abs.String())
		if ext == "" {
			// crawl
			go c.crawl(abs.String())
		} else {
			// add to download list
			c.addSrc(abs, ext)
		}
	})
}

func (c *crawler) addSrc(fileurl *url.URL, ext string) {
	c.sources = append(c.sources, dwndata{fileurl, ext})
}

func (c *crawler) Crawl() {
	c.crawl("https://www.gpo.gov/fdsys/bulkdata")
	c.wg.Wait()
}

func (c *crawler) Download() {
	c.wg.Add(len(c.sources))
	for _, s := range c.sources {
		go c.download(s.fileurl, s.ext)
	}
	c.wg.Wait()
}
