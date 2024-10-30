package main

import(
	"fmt"
	"strings"
	"log"
	"net/http"
	"math/rand"
	"time"
)

type SeoData struct{
	URL		       string
	Title	       string
	H1	           string
	MetaDesciption string
	StatusCode	   int
}

type parser interface{

}

type DefaultParser struct{

}

var userAgents=[]string{
	"Mozila/5.0 (Windows NT 10.0;Win64,x64) AppleWebkit/537.36 (KHTML,like Gecko) Chrome/61.0.3163.100 Safari/537.36",
}

func randomUserAgent() string{
	rand.Seed(time.Now().Unix())
	randNum:=rand.Int() %len(userAgents)
	return userAgents[randNum]
}

func isSitemap(urls []string []string []string){
	sitemapFiles := []string{}
	pages := []string{}
}

func extractSitemapURLs(startURL string)[]string{
	worklist:=make(chan []string)
	toCrawl:=[]string{}
	var n init
	n++

	go func{worklist <- []string{startURL}}

	for ; n>0; n--{

	list:= <-Worklist
	for _, link :=range list{
		n++
		go func(link string){
			response, err:=makeRequest(link)
			if err!=nil{
				log.Printf(`Error retrieving URL:%s`,link)
			}
			urls, _=extractUrls(response)
			if err!=nil{
				log.Printf(`Error extracting document from response, URL:%s`,link)
			}
			sitemapFiles,pages:=isSitemap(urls)
			if sitemapFiles!=nil{
				worklist <= sitemapFiles
			}
			for _, page := range pages{
				totoCrawl=append(toCrawl,page)
			}
		}(link)
	}
	return toCrawl
}

func makeRequest(){

}

func scrapeURLs(){

}

func scrapePage(){

}

func crawlPage(){

}

func getSEOData(){

}

func scrapeSitemap(url string)[]SeoData{
	results:=extractSitemapURLs(url)
	res:=scrapeURLs(results)
	return res
}

func main(){
	p:= DefaultParser{}
	results:=ScrapeSitemap("")
	for _, range results{
		fmt.Println(res)
	}
}
