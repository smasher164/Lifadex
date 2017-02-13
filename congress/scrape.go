package congress

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/smasher164/lifadex/container"
)

var progress container.IntStringBag
var trim = strings.TrimSpace

func init() {
	progress.Init([]struct {
		A int
		B string
	}{
		{0, "Introduced"},
		{1, "Passed House"},
		{2, "Passed Senate"},
		{3, "To President"},
		{4, "Became Law"},
	})
}

type Scraper interface {
	Scrape(io.Reader) error
}

type Resource struct {
	DocReader io.Reader
	Link      string
	doc       *goquery.Document
}

type Bill struct {
	Resource
	Title    string
	Progress int
	Summary  string
}

type Term struct {
	State      string
	District   string
	InCongress string
}

type Member struct {
	Resource
	Name    string
	Terms   []Term
	Website string
	Contact string
	Party   string
}

func (r *Resource) Get(url string) error {
	if res, err := http.Get(url); err != nil {
		return err
	} else {
		r.DocReader = res.Body
		r.Link = url
		return nil
	}
}

func (b *Bill) Scrape() error {
	if d, err := goquery.NewDocumentFromReader(b.DocReader); err != nil {
		return err
	} else {
		b.doc = d
	}
	b.Title = b.doc.Find(".legDetail").Nodes[0].FirstChild.Data
	p := b.doc.Find(".bill_progress > .selected").Nodes[0].FirstChild.Data
	b.Progress = progress.GetInt(trim(p))

	var sumBuf struct {
		bytes.Buffer
		err error
	}
	b.doc.Find("#bill-summary").Children().Each(func(i int, s *goquery.Selection) {
		if tag := goquery.NodeName(s); tag != "script" && tag != "div" {
			if h, err := goquery.OuterHtml(s); err != nil {
				sumBuf.err = err
				return
			} else {
				sumBuf.WriteString(h)
			}
		}
	})
	if sumBuf.err != nil {
		return sumBuf.err
	}
	sumBuf.Truncate(sumBuf.Len() - 1)
	b.Summary = sumBuf.String()
	return nil
}

func (m *Member) Scrape() error {
	if d, err := goquery.NewDocumentFromReader(m.DocReader); err != nil {
		return err
	} else {
		m.doc = d
	}
	m.Name = m.doc.Find(".legDetail").Nodes[0].FirstChild.Data
	m.Terms = []Term{}
	m.doc.Find(".lateral01 td").Each(func(i int, s *goquery.Selection) {
		switch stride := i % 3; stride {
		case 0:
			m.Terms = append(m.Terms, Term{State: trim(s.Text())})
		case 1:
			m.Terms[len(m.Terms)-1].District = trim(s.Text())
		case 2:
			m.Terms[len(m.Terms)-1].InCongress = trim(s.Text())
		}
	})
	m.Website = trim(m.doc.Find(".member_website + td").Text())
	if cs := m.doc.Find(".member_contact + td").Nodes; len(cs) > 0 {
		m.Contact = trim(cs[0].FirstChild.Data) + "\n" + trim(cs[0].LastChild.Data)
	}
	m.Party = trim(m.doc.Find(".member_party + td").Text())
	return nil
}
