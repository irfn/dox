package dox

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/flytam/filenamify"
	"golang.org/x/net/html"
)

type GeneratedPlantContent struct {
	UpdatedContent string
	UMLSrcFiles    []string
}

func findAttr(n *html.Node, key string) string {
	for _, at := range n.Attr {
		if at.Key == key {
			return at.Val
		}
	}
	return ""
}
func replace(n *html.Node, umlIdRelMap map[string]string) {
	if n.Type == html.ElementNode {
		if n.Data == "code" && findAttr(n, "class") == "language-plantuml" {
			id := findAttr(n, "id")
			preBlock := n.Parent
			img := &html.Node{
				Type: html.NodeType(html.ElementNode),
				Data: "img",
				Attr: []html.Attribute{
					{Key: "src", Val: umlIdRelMap[id]},
				},
			}
			preBlock.Parent.InsertBefore(img, preBlock)
			preBlock.Parent.RemoveChild(preBlock)
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		replace(child, umlIdRelMap)
	}
}

func replaceNodesWithNewImgs(content string, umlIdRelMap map[string]string) *GeneratedPlantContent {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil
	}

	replace(doc, umlIdRelMap)
	updatedContentBuffer := &bytes.Buffer{}
	html.Render(updatedContentBuffer, doc)

	rels := make([]string, 0, len(umlIdRelMap))
	for _, value := range umlIdRelMap {
		rels = append(rels, value)
	}
	updatedContent := updatedContentBuffer.String()
	return &GeneratedPlantContent{UpdatedContent: updatedContent, UMLSrcFiles: rels}
}

func getUMLBlocksFromHTML(content string) (string, map[string]string, error) {
	umlBlocks := make(map[string]string)
	doc, err := htmlquery.Parse(strings.NewReader(content))
	if err != nil {
		return content, nil, err
	}
	list := htmlquery.Find(doc, "//code")
	for _, code := range list {
		if htmlquery.SelectAttr(code, "class") == "language-plantuml" {
			h := sha256.New()
			umlContent := htmlquery.InnerText(code)
			h.Write([]byte(umlContent))
			fileNameOptions := filenamify.Options{Replacement: "-", MaxLength: 30}
			encodedStr := hex.EncodeToString(h.Sum(nil))

			uniqueid, err := filenamify.Filenamify(encodedStr, fileNameOptions)
			if err != nil {
				return content, nil, err
			}
			id := url.QueryEscape(uniqueid)
			code.Attr = append(code.Attr, html.Attribute{Key: "id", Val: id})
			umlBlocks[id] = umlContent
		}
	}
	updatedContent := htmlquery.OutputHTML(doc, true)
	return updatedContent, umlBlocks, nil
}

func generatePlantImageSrcFilesAndReplaceContent(content string, file string, repoRoot string, jarPath string) (*GeneratedPlantContent, error) {

	tmpfileDir := fmt.Sprintf("%s/.plantuml", repoRoot)
	os.MkdirAll(tmpfileDir, os.ModePerm)

	updatedContent, umlBlocks, err := getUMLBlocksFromHTML(content)
	if err != nil {
		return nil, err
	}
	imageSrcFiles := make(map[string]string)

	for umlId, uml := range umlBlocks {
		umlcontent := []byte(uml)

		umlSrcPath := filepath.Join(tmpfileDir, fmt.Sprintf("%s.puml", umlId))
		// write the whole body at once
		err = os.WriteFile(umlSrcPath, umlcontent, 0644)
		if err != nil {
			return nil, err
		}

		// Execute the command and store the output in a file
		outputFilePath := filepath.Join(tmpfileDir, fmt.Sprintf("%s.png", umlId))
		rel, err := filepath.Rel(file, outputFilePath)
		if err != nil {
			return nil, err
		}
		cmd := exec.Command("java", "-jar", jarPath, umlSrcPath)
		err = cmd.Run()
		if err != nil {
			return nil, err
		}
		relPath, found := strings.CutPrefix(rel, "../")
		if !found {
			panic("some bug")
		}

		imageSrcFiles[umlId] = relPath
	}
	return replaceNodesWithNewImgs(updatedContent, imageSrcFiles), nil
}
