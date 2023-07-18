package main

import (
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/bigkevmcd/go-configparser"
)

const (
	TRUE_HOST = "exhentai.org"
	COOKIE    = `ipb_member_id=4761956; ipb_pass_hash=16f4dc00b025b2e51f59e2a2365d4490; yay=louder; igneous=fb28508a0; ls=dm_1`
)

var host map[string]([]string)
var cookie string
var trueHost string
var ipArr []string
var re *regexp.Regexp

func main() {

	errNotice := func(err error) {
		log.Println("程序退出了，一般来说并不会显示这行，请尝试关掉之前打开的此程序")
		log.Println("报错信息为：", err)
		time.Sleep(time.Second * 30)
		os.Exit(-1)
	}

	re = regexp.MustCompile(`Domain=[\w\.]*;`)
	host = make(map[string][]string)
	// cookie = COOKIE
	// trueHost = TRUE_HOST

	p, err := configparser.NewConfigParserFromFile("localhentai.ini")
	if err != nil {
		log.Println(`配置文件读取失败：`, err)
		errNotice(err)
	}
	trueHost, err = p.Get("config", "host")
	if err != nil {
		log.Println(`配置文件读取失败，未在[config]中指定host：`, err)
		errNotice(err)
	}
	listen, err := p.Get("config", "listen")
	if err != nil {
		log.Println(`配置文件读取失败，未在[config]中指定listen：`, err)
		errNotice(err)
	}
	log.Println(`监听地址：`, listen)
	cookie, err = p.Get(trueHost, "cookie")
	if err != nil {
		if trueHost == "exhentai.org" {
			log.Println(`没有找到cookie：`, err)
			log.Println("使用默认配置(公车号)")
			cookie = COOKIE
		}
	}
	ips, err := p.Get(trueHost, "ips")
	if err != nil {
		log.Println(fmt.Sprintf(`配置文件读取失败，未在[%s]中指定ips(使用半角逗号,隔开)：`, trueHost), err)
		errNotice(err)
	}
	ipArr = strings.Split(ips, ",")
	for k, v := range ipArr {
		ipArr[k] = strings.Trim(v, " ")
	}
	host[trueHost] = ipArr

	fmt.Println("*******************************************")
	fmt.Println("二次配布，修改，一切自由(明明是屎还这么嚣张……)")
	fmt.Println("*******************************************")

	http.HandleFunc("/", httpHandler(trueHost))
	err = http.ListenAndServe(listen, nil)

	errNotice(err)
}
func httpHandler(trueHost string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		makeRequestWithoutSNI(w, r, trueHost)
	}
}
func getIPLocaly(s string) string {
	return host[s][rand.Intn(len(host[s]))]
}
func getPlainTextReader(body io.ReadCloser, encoding string) io.ReadCloser {
	switch encoding {
	case "gzip":
		reader, err := gzip.NewReader(body)
		if err != nil {
			log.Fatal("error decoding gzip response", reader)
		}
		return reader
	case "br":
		reader := brotli.NewReader(body)
		if reader == nil {
			log.Fatal("error decoding br response", reader)
		}
		return io.NopCloser(reader)
	default:
		return body
	}
}
func makeRequestWithoutSNI(w http.ResponseWriter, r *http.Request, trueHost string) []byte {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: tr,
	}

	newUrl := r.URL
	newUrl.Host = getIPLocaly(trueHost) // IP
	newUrl.Scheme = "https"

	// the search is done by get,
	// so only use get for prevent post with the provided account
	req, err := http.NewRequest("GET", newUrl.String(), nil)
	if err != nil {
		fmt.Println(`Error On NewRequest`, err)
		return nil
	}
	// it seems that override the request here...
	req.Header = r.Header
	req.Method = r.Method // wait?
	req.Body = r.Body     //
	req.Host = trueHost   //Host
	req.Header.Set("Cookie", cookie)

	// never mind...
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(`Error On Do Request`, err)
		return nil
	}
	defer resp.Body.Close()

	// fmt.Println(resp.Header.Get("Content-Encoding"))
	// fmt.Println(resp.Header.Get("Content-Type"))

	// fmt.Println(resp.Header)

	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/html") ||
		strings.HasPrefix(contentType, "application/javascript") ||
		strings.HasPrefix(contentType, "text/css") {

		body := getPlainTextReader(resp.Body, resp.Header.Get("Content-Encoding"))
		text, err := ioutil.ReadAll(body)
		if err != nil {
			fmt.Println(`Error On Read Body`, err)
			return nil
		}

		textReplaced := strings.Replace(string(text), "https://exhentai.org/", "/", -1)
		if trueHost == "exhentai.org" && strings.HasPrefix(r.URL.Path, `/s/`) {
			textReplaced = addWaterFallViewButton(textReplaced)
		}

		for k, v := range resp.Header {
			if k == "Content-Length" {
				continue
			}
			for _, vi := range v {
				w.Header().Add(k, vi)
			}
		}
		w.Header().Del("Content-Encoding")

		// w.WriteHeader(http.StatusOK)
		w.Write([]byte(textReplaced))

	} else {
		for k, v := range resp.Header {
			if k == "Content-Length" {
				continue
			}
			for _, vi := range v {
				if k == "Set-Cookie" {
					vi = string(re.ReplaceAll([]byte(vi), []byte{}))
				}
				w.Header().Add(k, vi)
			}
		}

		// w.WriteHeader(http.StatusOK)
		io.Copy(w, resp.Body)
	}

	return nil
}
func addWaterFallViewButton(html string) string {
	return strings.Replace(html, "<body>", `<body>
	<div style="
	  height: 60px;
	  width: 100px;
	  text-align: center;
	  /* background-color: violet; */
	  position: fixed;
	  right: 20px; 
	  top: 20px;
	  z-index: 99;
	  display: table-cell;
	  vertical-align: middle;
	  /* float: right; */
	">
	  <button id="waterfall" style="
		width: 100%;    
		height: 100%;
		font-size: x-large;
	  ">
		下拉式
	  </button>
	</div>
  <script type="text/javascript">
	async function execWaterfall(){
		console.log('!');
		document.getElementById("waterfall").remove();
		let pn = document.createElement('div');
		let lp = location.href;
		let ln = location.href;
		const element = document.getElementById('i1');
		element.appendChild(pn);
		let hn = document.getElementById('next').href;
		while (hn != ln) {
		  let doc;
		  while(!doc) {
			doc = await fetch(hn).then(resp => resp.text())			
			  .then(data => {
			    console.log(data);
			    let parser = new DOMParser();
			    let doc = parser.parseFromString(data, "text/html");
			    return doc;
			  });
			}
		  console.log(doc);
		  let img = document.createElement('img');
		  let element = doc.getElementById('img');
		  if (element) {
			img.src = element.src;
			pn.appendChild(img);
			ln = hn;
			hn = doc.getElementById('next').href;
		  }
		}
		let p = document.createElement('p');
		p.innerHTML = hn;
	  }
	document.getElementById("waterfall").addEventListener("click", execWaterfall, false); 
	</script>`, 1)
}
