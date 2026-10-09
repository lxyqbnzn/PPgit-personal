package platform

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"
)

func PPGitProcesses() ([]Process, error) {
	// @Copyright Electric Reverse

	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(snapshot)
	var entry syscall.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	queryImage := syscall.NewLazyDLL("kernel32.dll").NewProc("QueryFullProcessImageNameW")
	var processes []Process
	for err = syscall.Process32First(snapshot, &entry); err == nil; err = syscall.Process32Next(snapshot, &entry) {
		name := syscall.UTF16ToString(entry.ExeFile[:])
		if !strings.EqualFold(name, "ppgit.exe") && !strings.EqualFold(name, "ppgit-windows-386.exe") {
			continue
		}
		handle, openErr := syscall.OpenProcess(0x1000, false, entry.ProcessID)
		if openErr != nil {
			continue
		}
		buffer := make([]uint16, 32768)
		size := uint32(len(buffer))
		ok, _, _ := queryImage.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
		syscall.CloseHandle(handle)
		if ok != 0 {
			processes = append(processes, Process{PID: int(entry.ProcessID), Executable: syscall.UTF16ToString(buffer[:size])})
		}
	}
	if !errors.Is(err, syscall.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return processes, nil
}
