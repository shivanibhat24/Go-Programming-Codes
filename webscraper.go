package main

import(
	
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

func randomUserAgent(){

}

func extractSitemapURLs(startURL string)[]string{
	worklist:=make(chan []string)
	toCrawl:=[]string{}

	go func{worklist <- []string{startURL}}

	for ; n>0; n--

	list:= <-Worklist

	for _, link :=range list{
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
		}
	}
	makeRequest
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
