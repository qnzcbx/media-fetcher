package main

import (
	"fmt"
	"golang.org/x/net/publicsuffix"
)

func main() {
	check("com")
	check("example.com")
	check("co.uk")
	check("bbc.co.uk")
	check("com.cn")
	check("a.com.cn")
}

func check(domain string) {
	ps, _ := publicsuffix.PublicSuffix(domain)
	fmt.Printf("%s -> %s (is PS: %v)\n", domain, ps, ps == domain)
}
