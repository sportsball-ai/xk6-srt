package srt

import "fmt"

func ReportError(err error) {
	if err != nil {
		fmt.Println(err.Error())
	}
}
