package dox

import (
	"errors"
	"os"

	"github.com/jesselang/go-confluence"
	"github.com/spf13/viper"
	"github.com/jesselang/dox/pkg/source"
)

func Publish(file string, rootID string, repoRoot string, dryRun bool) (id string, err error) {
	uri := viper.GetString("uri")
	if len(uri) == 0 {
		return id, errors.New("uri must be set in config")
	}

	space := viper.GetString("space")
	if len(space) == 0 {
		return id, errors.New("space must be set in config")
	}

	repoBrowseURL := viper.GetString("repo_browse_url")
	if len(repoBrowseURL) == 0 {
		return id, errors.New("repo_browse_url must be set in config")
	}

	username := os.Getenv("DOX_USERNAME")
	if len(username) == 0 {
		return id, errors.New("DOX_USERNAME must be set")
	}

	password := os.Getenv("DOX_PASSWORD")
	if len(password) == 0 {
		return id, errors.New("DOX_PASSWORD must be set")
	}

	wiki, err := confluence.NewWiki(
		uri,
		confluence.BasicAuth(
			username,
			password,
		),
	)
	if err != nil {
		return
	}

	src, err := source.New(file, source.Opts{StripComments: true, TrimSpace: true})
	if err != nil {
		return
	}

	var c *confluence.Content
	if dryRun {
		return src.ID(), nil
	} else if src.ID() == "" {
		// NEW
		c = &confluence.Content{
			ID:    src.ID(),
			Type:  "page",
			Title: src.Title(),
		}

		if rootID != "" {
			c.Ancestors = []confluence.ContentAncestor{{ID: rootID}}
		}
		c.Body.Storage.Value = src.Output()
		c.Body.Storage.Representation = "storage"
		c.Space.Key = space
		c.Version.Number = 1

		c, err = wiki.CreateContent(c)
		if err != nil {
			// TODO: confluence does not support duplicate title in a space
			return "", err
		}

		err = src.SetID(c.ID)
		if err != nil {
			return "", err
		}

		imageSrcFiles, err := getImageSrcFiles(c.Body.Storage.Value, file)
		if err != nil {
			return "", err
		}

		if len(imageSrcFiles) != 0 {
			c.Body.Storage.Value, err = replaceImagesWithAttachments(imageSrcFiles, file, c.Body.Storage.Value, c.ID, wiki, uri)
			if err != nil {
				return "", err
			}

			c.Version.Number += 1
			c, err = wiki.UpdateContent(c)
			if err != nil {
				return "", err
			}
		}

	} else {
		sourceOutput := src.Output()

		c, err = wiki.GetContent(src.ID(),
			[]string{"body.storage", "space", "version"})
		if err != nil {
			// TODO: handle 404 where dox id exists in source, but published page does not
			return "", err
		}

		imageSrcFiles, err := getImageSrcFiles(sourceOutput, file)
		if err != nil {
			return "", err
		}

		pageContent, err := replaceImagesWithAttachments(imageSrcFiles, file, sourceOutput, c.ID, wiki, uri)
		if err != nil {
			return "", err
		}

		pageContent, err = replaceRelativeLinks(file, pageContent, uri, repoBrowseURL, repoRoot)
		if err != nil {
			return "", err
		}

		if c.Body.Storage.Value != pageContent {
			c.Body.Storage.Value = pageContent
			c.Version.Number += 1

			c, err = wiki.UpdateContent(c)
			if err != nil {
				return "", err
			}
		}
	}
	return c.ID, nil
}
