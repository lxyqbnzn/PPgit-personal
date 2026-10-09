package core

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func isLink(i os.FileInfo) bool {
	if i.Mode()&os.ModeSymlink != 0 {
		return true
	}
	s, ok := i.Sys().(*syscall.Win32FileAttributeData)
	return ok && s.FileAttributes&0x400 != 0
}

var getFileInformationByHandleEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFileInformationByHandleEx")

type fileBasicInformation struct {
	CreationTime   int64
	LastAccessTime int64
	LastWriteTime  int64
	ChangeTime     int64
	FileAttributes uint32
	Padding        uint32
}

func metadataSignature(path string, info os.FileInfo) (string, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	handle, err := syscall.CreateFile(name, 0x80, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return "", &os.PathError{Op: "read file identity", Path: path, Err: err}
	}
	defer syscall.CloseHandle(handle)
	var basic fileBasicInformation
	ok, _, callErr := getFileInformationByHandleEx.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&basic)), unsafe.Sizeof(basic))
	if ok == 0 {
		return "", &os.PathError{Op: "read file change time", Path: path, Err: callErr}
	}
	if basic.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return "", fmt.Errorf("reparse point is not allowed: %s", path)
	}
	var identity syscall.ByHandleFileInformation
	if err = syscall.GetFileInformationByHandle(handle, &identity); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d:%d:%d:%d", signature(info), basic.ChangeTime, identity.VolumeSerialNumber, identity.FileIndexHigh, identity.FileIndexLow), nil
}

func signature(i os.FileInfo) string {
	s, _ := i.Sys().(*syscall.Win32FileAttributeData)
	if s != nil {
		return fmt.Sprintf("%d:%d:%d:%d:%d", i.Size(), i.ModTime().UnixNano(), s.CreationTime.HighDateTime, s.CreationTime.LowDateTime, i.Mode())
	}
	return fmt.Sprintf("%d:%d:%d", i.Size(), i.ModTime().UnixNano(), i.Mode())
}
