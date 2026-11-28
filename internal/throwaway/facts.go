package throwaway

import (
	"fmt"
	"os"
	"reflect"
	"time"
)

// InspectFile reads identity and timestamps without opening the contents.
// The native stat layout differs between the supported test and grade hosts;
// inspect its exported metadata fields here so this derivation stays untagged.
// An unknown layout is an engine error, never a zero-valued observation.
func InspectFile(name string) (FileFacts, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return FileFacts{}, err
	}
	return nativeFileFacts(info.Sys(), info.ModTime())
}

func nativeFileFacts(metadata any, modified time.Time) (FileFacts, error) {
	stat := reflect.ValueOf(metadata)
	if stat.Kind() == reflect.Pointer && !stat.IsNil() {
		stat = stat.Elem()
	}
	if stat.Kind() != reflect.Struct {
		return FileFacts{}, fmt.Errorf("unsupported file metadata")
	}
	device, okDevice := unsignedStatField(stat.FieldByName("Dev"))
	inode, okInode := unsignedStatField(stat.FieldByName("Ino"))
	access := stat.FieldByName("Atim")
	if !access.IsValid() {
		access = stat.FieldByName("Atimespec")
	}
	if !okDevice || !okInode || access.Kind() != reflect.Struct {
		return FileFacts{}, fmt.Errorf("unsupported file identity or access time")
	}
	sec, nsec := access.FieldByName("Sec"), access.FieldByName("Nsec")
	if !sec.CanInt() || !nsec.CanInt() {
		return FileFacts{}, fmt.Errorf("unsupported access time precision")
	}
	return FileFacts{Access: time.Unix(sec.Int(), nsec.Int()), Modified: modified, Device: device, Inode: inode}, nil
}
func unsignedStatField(field reflect.Value) (uint64, bool) {
	if field.CanUint() {
		return field.Uint(), true
	}
	if field.CanInt() {
		return uint64(field.Int()), true
	}
	return 0, false
}
