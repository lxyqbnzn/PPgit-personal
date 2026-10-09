//go:build !windows

package core

import (
	"fmt"
	"os"
	"reflect"
)

func isLink(i os.FileInfo) bool { return i.Mode()&os.ModeSymlink != 0 }

func metadataSignature(_ string, info os.FileInfo) (string, error) { return signature(info), nil }

func signature(i os.FileInfo) string {
	v := reflect.Indirect(reflect.ValueOf(i.Sys()))
	extra := ""
	if v.IsValid() && v.Kind() == reflect.Struct {
		for _, n := range []string{"Dev", "Ino", "Ctim", "Ctimespec"} {
			f := v.FieldByName(n)
			if f.IsValid() {
				extra += fmt.Sprint(f.Interface()) + ":"
			}
		}
	}
	return fmt.Sprintf("%d:%d:%d:%s", i.Size(), i.ModTime().UnixNano(), i.Mode(), extra)
}
