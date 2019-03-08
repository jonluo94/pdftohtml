package docconv

import (
	"fmt"
	"io"
	"strings"
	"time"
	"os/exec"
	"io/ioutil"
	"strconv"
	"bytes"
	"golang.org/x/net/html"
	"errors"
	"regexp"
)

func ConvertPDF(r io.Reader) (string, map[string]string, error) {

	f, err := NewLocalFile(r, "/tmp", "pdf-convert-")
	if err != nil {
		return "", nil, fmt.Errorf("error creating local file: %v", err)
	}
	defer f.Done()

	bodyResult, metaResult, convertErr := ConvertPDFHtml(f.Name())
	if convertErr != nil {
		return "", nil, convertErr
	}
	if bodyResult.err != nil {
		return "", nil, bodyResult.err
	}
	if metaResult.err != nil {
		return "", nil, metaResult.err
	}
	return bodyResult.body, metaResult.meta, nil

}

// Meta data
type MetaResult struct {
	meta map[string]string
	err  error
}

type BodyResult struct {
	body string
	err  error
}

// Convert PDF to Html
func ConvertPDFHtml(file string) (BodyResult, MetaResult, error) {

	htmlFile := file + ".html"

	metaResult := MetaResult{meta: make(map[string]string)}
	bodyResult := BodyResult{}
	mr := make(chan MetaResult, 1)
	go func() {
		metaStr, err := exec.Command("pdfinfo", file).Output()
		if err != nil {
			metaResult.err = err
			mr <- metaResult
			return
		}

		// Parse meta output
		for _, line := range strings.Split(string(metaStr), "\n") {
			if parts := strings.SplitN(line, ":", 2); len(parts) > 1 {
				metaResult.meta[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		// Convert parsed meta
		if tmp, ok := metaResult.meta["Author"]; ok {
			metaResult.meta["Author"] = tmp
		}
		if tmp, ok := metaResult.meta["ModDate"]; ok {
			if t, err := time.Parse(time.ANSIC, tmp); err == nil {
				metaResult.meta["ModifiedDate"] = fmt.Sprintf("%d", t.Unix())
			}
		}
		if tmp, ok := metaResult.meta["CreationDate"]; ok {
			if t, err := time.Parse(time.ANSIC, tmp); err == nil {
				metaResult.meta["CreatedDate"] = fmt.Sprintf("%d", t.Unix())
			}
		}

		mr <- metaResult
	}()

	br := make(chan BodyResult, 1)
	go func() {
		var body bytes.Buffer

		pageInfo, err := exec.Command("pdftohtml", "-c", file, htmlFile).Output()
		if err != nil {
			bodyResult.err = err
		}
        pages := strings.Count(string(pageInfo),"\n")
		for i := 1;i <= pages;i++{
			data, err := ioutil.ReadFile(file+"-"+strconv.Itoa(i)+".html")
			if err != nil {
				fmt.Errorf("File reading error", err)
				return
			}

			doc, _ := html.Parse(strings.NewReader(string(data)))
			bodyData, err := getHtmlBody(doc)
			if err != nil {
				fmt.Errorf("read html fail", err)
				return
			}
			body.WriteString(bodyData)
			if i != pages {
				body.WriteString("\f")
			}
		}

		bodyResult.body = body.String()

		br <- bodyResult
	}()

	return <-br, <-mr, nil
}

func getHtmlBody(doc *html.Node) (string, error) {
	var b *html.Node
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			b = n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	if b != nil {
		var buf bytes.Buffer
		w := io.Writer(&buf)
		html.Render(w, b)

		//过滤 <img/> 标签
		reg, err := regexp.Compile("<img\\s[^>]+/>")
		if err != nil {
			fmt.Errorf("regexp fail", err)
		}
		str := reg.ReplaceAllString(buf.String(), "")
		return str, nil
	}
	return "", errors.New("Missing <body> in the node tree")
}

