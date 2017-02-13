package main

import (
	"fmt"
	"log"

	"github.com/smasher164/lifadex/congress"
)

func main() {
	// var b congress.Bill
	// if err := b.Get("https://www.congress.gov/bill/114th-congress/house-bill/158"); err != nil {
	// 	log.Fatalf("13: Get() error %v\n", err)
	// }
	// if err := b.Scrape(); err != nil {
	// 	log.Fatalf("16: Scrape() error %v\n", err)
	// }
	// fmt.Println(b.Summary)

	var m congress.Member
	if err := m.Get("https://www.congress.gov/member/charles-schumer/S000148"); err != nil {
		log.Fatalln(err)
	}
	if err := m.Scrape(); err != nil {
		log.Fatalln(err)
	}
	fmt.Println(m)
}
