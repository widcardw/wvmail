package services

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// replaceComposeImageSources turns editor preview data URLs into stable cid:
// references. The editor keeps its preview HTML untouched so a failed send can
// be retried without losing the locally rendered image.
func replaceComposeImageSources(sourceHTML string, images []ComposeAttachment) (string, error) {
	if !strings.Contains(sourceHTML, "data-compose-image-id") {
		return sourceHTML, nil
	}

	contentIDs := make(map[string]string, len(images))
	for _, image := range images {
		if image.ID != "" && image.ContentID != "" {
			contentIDs[image.ID] = image.ContentID
		}
	}

	context := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(sourceHTML), context)
	if err != nil {
		return "", fmt.Errorf("parse compose HTML: %w", err)
	}

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.DataAtom == atom.Img {
			imageID := attributeValue(node, "data-compose-image-id")
			if contentID := contentIDs[imageID]; contentID != "" {
				setAttribute(node, "src", "cid:"+contentID)
				setAttribute(node, "data-content-id", contentID)
				removeAttribute(node, "data-compose-image-id")
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	var rendered strings.Builder
	for _, node := range nodes {
		walk(node)
		if err := html.Render(&rendered, node); err != nil {
			return "", fmt.Errorf("render compose HTML: %w", err)
		}
	}
	return rendered.String(), nil
}

func attributeValue(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func setAttribute(node *html.Node, key, value string) {
	for i := range node.Attr {
		if node.Attr[i].Key == key {
			node.Attr[i].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: key, Val: value})
}

func removeAttribute(node *html.Node, key string) {
	attrs := node.Attr[:0]
	for _, attr := range node.Attr {
		if attr.Key != key {
			attrs = append(attrs, attr)
		}
	}
	node.Attr = attrs
}
