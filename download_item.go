package main

type DownloadItem struct {
	fileName string
	url      string
}

func (i *DownloadItem) isValid() bool {
	return i.fileName != "" && i.url != ""
}
