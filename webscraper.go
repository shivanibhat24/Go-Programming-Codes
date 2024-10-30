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
	getSEOData(resp *http.Response)(SeoData, error)
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

func isSitemap(urls []string)( []string, []string){
	sitemapFiles := []string{}
	pages := []string{}
	for _, range(urls){
		foundSitemap := string.Contains(page, "xml")
		foundSitemap ==true{
			fmt.Println("Found Sitemap",page)
			sitesitemapFiles=append(sitemapFiles,page)
		}else{
			pages=append(pages,page)
		}
	}
	return sitemapFiles
}

func extractSitemapURLs(startURL string)[]string{
	worklist:=make(chan []string)
	toCrawl:=[]string{}
	var n init
	n++

	go func{worklist <- []string{startURL}}

	for ; n>0; n--{

	list:= <-worklist
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

func makeRequest(url string)(*http.Response,err){
	client:=http.Client{
		Timeout: 10*time.Second,
	}
	req,err:=http.NewRequest("GET",url,nil)
	req.Header.Set("User-Agent",randomUserAgent())
	if err!=nil{
		return nil,err
	}
	res, client.Do(req)
	if err!=nil{
		return nil,err
	}
	return res,nil
}

func extractUrls(response *http.Response)([]string,error){
	doc,err:=goquery.NewDocumentFromResponse(response)
	if err!=nil{
		return nil,err
	}
	results:=[]string{}
	sel=doc.Find("loc")
	for i:=range sel.Nodes{
		loc:=sel.Eq(i)
		result:=loc.Text()
		results=append(results, result)
	}
	return results,nil
}

func scrapeURLs(urls []string,parser Parser,concurrency int)[]SeoData{
	results:= extractSitemapURLs(url)
	res:=scrapeURLs(results,parser,concurrency)
	return res
}

func scrapeURL(urls []string,parser Parser,concurrency int)[]SeoData{
	token:=make(chan struct{},concurrency)
	var n int
	worklist:= make(chan []string)
	results:=[]SeoData{}

	go func(){worklist <- urls}()
	for; n>0;n--{
		list:=<-worklist
		for _, url:=range list{
			if url!=""{
				n++
				go func(url string, token chan struct{}){
					log.Printf("Requesting URL:%s",url)
					res,err:=scrapePage(url,tokens,parser)
					if err!=nil{
						log.Printf("Encountered an error, URL:%s",url)
					}else{
						results=append(results,res)
					}
					worklist <- []string{}
				}(url,tokens)
			}
		}
	}
}

func scrapePage(url string,token chan struct{}, parser Parser)(SeoData,error){
	res,err=scrapePage(url)
	if err!=nil{
		return SeoData{},err
	}
	data,err:=parser.getSEOData(res)
	if err!=nil{
		return SeoData{},err
	}
	return data,nil
}

func crawlPage(url string,tokens chan struct{})(*http.Response,error){
	tokens<- struct{}{}
	resp,err:=makeRequest(url)
	<-tokens
	if err!=nil{
		return nil,err
	}
	return resp,err
}

func (d DefaultParser)getSEOData(resp *http.Response)(SeoData,error){
	doc,err:=goquery.NewDocumentFromResponse(resp)
	if err!=nil{
		return SeoData{},err
	}
	result:=SeoData{}
	result.URL=resp.Request.URL.String()
	result.StatusCode=resp.StatusCode
	result.Title=doc.Find("title").First().Text()
	result.H1=doc.Find("h1").First().Text()
	result.MetaDesciption=doc.Find("meta[name^=description]".Attr("content"))
	return result,nil
}

func ScrapeSitemap(url string, parser Parser,concurrency int)[]SeoData{
	results:=extractSitemapURLs(url)
	res:=scrapeURLs(results,parser,concurrency)
	return res
}

func main(){
	p:= DefaultParser{}
	results:=ScrapeSitemap("https//www.quicksprout.com/sitemap.xml",p,10)
	for _, range results{
		fmt.Println(res)
	}
}
