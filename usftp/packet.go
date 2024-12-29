package usftp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
)

var (
	//errLongPacket            = errors.New("packet too long")
	errShortPacket = errors.New("packet too short")
	//errUnknownExtendedPacket = errors.New("unknown extended packet")

	bigEnd_ = binary.BigEndian
)

func marshalString(b []byte, v string) []byte {
	return append(bigEnd_.AppendUint32(b, uint32(len(v))), v...)
}

func marshalFileInfo(b []byte, fi os.FileInfo) []byte {
	// attributes variable struct, and also variable per protocol version
	// spec version 3 attributes:
	// uint32   flags
	// uint64   size           present only if flag SSH_FILEXFER_ATTR_SIZE
	// uint32   uid            present only if flag SSH_FILEXFER_ATTR_UIDGID
	// uint32   gid            present only if flag SSH_FILEXFER_ATTR_UIDGID
	// uint32   permissions    present only if flag SSH_FILEXFER_ATTR_PERMISSIONS
	// uint32   atime          present only if flag SSH_FILEXFER_ACMODTIME
	// uint32   mtime          present only if flag SSH_FILEXFER_ACMODTIME
	// uint32   extended_count present only if flag SSH_FILEXFER_ATTR_EXTENDED
	// string   extended_type
	// string   extended_data
	// ...      more extended data (extended_type - extended_data pairs),
	// 	   so that number of pairs equals extended_count

	flags, fileStat := fileStatFromInfo(fi)

	b = bigEnd_.AppendUint32(b, flags)

	return marshalFileStat(b, flags, fileStat)
}

func marshalFileStat(b []byte, flags uint32, fileStat *FileStat) []byte {
	if flags&sshFileXferAttrSize != 0 {
		b = bigEnd_.AppendUint64(b, fileStat.Size)
	}
	if flags&sshFileXferAttrUIDGID != 0 {
		b = bigEnd_.AppendUint32(b, fileStat.UID)
		b = bigEnd_.AppendUint32(b, fileStat.GID)
	}
	if flags&sshFileXferAttrPermissions != 0 {
		b = bigEnd_.AppendUint32(b, fileStat.Mode)
	}
	if flags&sshFileXferAttrACmodTime != 0 {
		b = bigEnd_.AppendUint32(b, fileStat.Atime)
		b = bigEnd_.AppendUint32(b, fileStat.Mtime)
	}

	if flags&sshFileXferAttrExtended != 0 {
		b = bigEnd_.AppendUint32(b, uint32(len(fileStat.Extended)))

		for _, attr := range fileStat.Extended {
			b = marshalString(b, attr.ExtType)
			b = marshalString(b, attr.ExtData)
		}
	}

	return b
}

//func marshalStatus(b []byte, err StatusError) []byte {
//	b = bigEnd_.AppendUint32(b, err.Code)
//	b = marshalString(b, err.msg)
//	b = marshalString(b, err.lang)
//	return b
//}

func marshal(b []byte, v any) []byte {
	switch v := v.(type) {
	case nil:
		return b
	case uint8:
		return append(b, v)
	case uint32:
		return bigEnd_.AppendUint32(b, v)
	case uint64:
		return bigEnd_.AppendUint64(b, v)
	case string:
		return marshalString(b, v)
	case []byte:
		return append(b, v...)
	case os.FileInfo:
		return marshalFileInfo(b, v)
	default:
		switch d := reflect.ValueOf(v); d.Kind() {
		case reflect.Struct:
			for i, n := 0, d.NumField(); i < n; i++ {
				b = marshal(b, d.Field(i).Interface())
			}
			return b
		case reflect.Slice:
			for i, n := 0, d.Len(); i < n; i++ {
				b = marshal(b, d.Index(i).Interface())
			}
			return b
		default:
			panic(fmt.Sprintf("marshal(%#v): cannot handle type %T", v, v))
		}
	}
}

func unmarshalUint32(b []byte) (v uint32, outB []byte) {
	v = binary.BigEndian.Uint32(b)
	return v, b[4:]
}

func unmarshalUint32Safe(b []byte) (uint32, []byte, error) {
	var v uint32
	if len(b) < 4 {
		return 0, nil, errShortPacket
	}
	v, b = unmarshalUint32(b)
	return v, b, nil
}

func unmarshalUint64(b []byte) (v uint64, outB []byte) {
	v = binary.BigEndian.Uint64(b)
	return v, b[8:]
}

func unmarshalUint64Safe(b []byte) (uint64, []byte, error) {
	var v uint64
	if len(b) < 8 {
		return 0, nil, errShortPacket
	}
	v, b = unmarshalUint64(b)
	return v, b, nil
}

func unmarshalString(b []byte) (string, []byte) {
	n, b := unmarshalUint32(b)
	return string(b[:n]), b[n:]
}

func unmarshalStringSafe(b []byte) (string, []byte, error) {
	n, b, err := unmarshalUint32Safe(b)
	if err != nil {
		return "", nil, err
	}
	if int64(n) > int64(len(b)) {
		return "", nil, errShortPacket
	}
	return string(b[:n]), b[n:], nil
}

func unmarshalAttrs(b []byte) (*FileStat, []byte, error) {
	flags, b, err := unmarshalUint32Safe(b)
	if err != nil {
		return nil, b, err
	}
	return unmarshalFileStat(flags, b)
}

func unmarshalFileStat(flags uint32, b []byte) (*FileStat, []byte, error) {
	var fs FileStat
	var err error

	if flags&sshFileXferAttrSize == sshFileXferAttrSize {
		fs.Size, b, err = unmarshalUint64Safe(b)
		if err != nil {
			return nil, b, err
		}
	}
	if flags&sshFileXferAttrUIDGID == sshFileXferAttrUIDGID {
		fs.UID, b, err = unmarshalUint32Safe(b)
		if err != nil {
			return nil, b, err
		}
		fs.GID, b, err = unmarshalUint32Safe(b)
		if err != nil {
			return nil, b, err
		}
	}
	if flags&sshFileXferAttrPermissions == sshFileXferAttrPermissions {
		fs.Mode, b, err = unmarshalUint32Safe(b)
		if err != nil {
			return nil, b, err
		}
	}
	if flags&sshFileXferAttrACmodTime == sshFileXferAttrACmodTime {
		fs.Atime, b, err = unmarshalUint32Safe(b)
		if err != nil {
			return nil, b, err
		}
		fs.Mtime, b, err = unmarshalUint32Safe(b)
		if err != nil {
			return nil, b, err
		}
	}
	if flags&sshFileXferAttrExtended == sshFileXferAttrExtended {
		var count uint32
		count, b, err = unmarshalUint32Safe(b)
		if err != nil {
			return nil, b, err
		}

		ext := make([]StatExtended, count)
		for i := uint32(0); i < count; i++ {
			var typ string
			var data string
			typ, b, err = unmarshalStringSafe(b)
			if err != nil {
				return nil, b, err
			}
			data, b, err = unmarshalStringSafe(b)
			if err != nil {
				return nil, b, err
			}
			ext[i] = StatExtended{
				ExtType: typ,
				ExtData: data,
			}
		}
		fs.Extended = ext
	}
	return &fs, b, nil
}

func unmarshalStatus(b []byte) error {
	code, b := unmarshalUint32(b)
	msg, b, _ := unmarshalStringSafe(b)
	lang, _, _ := unmarshalStringSafe(b)
	return &StatusError{
		Code: code,
		msg:  msg,
		lang: lang,
	}
}

type (
	appendable_ interface {
		appendTo([]byte) ([]byte, error)
	}

	idAwarePkt_ interface {
		appendable_
		id() uint32
		setId(id uint32)
	}

	idPkt_ struct {
		ID uint32
	}
)

func (p *idPkt_) id() uint32      { return p.ID }
func (p *idPkt_) setId(id uint32) { p.ID = id }

// sendPacket marshals pkt according to RFC 4234.
func sendPacket(w io.Writer, buff []byte, pkt appendable_) (err error) {
	outBuff, err := pkt.appendTo(buff[4:4])
	if err != nil {
		return fmt.Errorf("binary marshaller failed: %w", err)
	}
	length := len(outBuff)
	outBuff = buff[:4+len(outBuff)]
	binary.BigEndian.PutUint32(outBuff[:4], uint32(length))

	_, err = w.Write(outBuff)
	if err != nil {
		return fmt.Errorf("failed to send packet: %w", err)
	}
	return
}

type extensionPair struct {
	Name string
	Data string
}

func unmarshalExtensionPair(b []byte) (extensionPair, []byte, error) {
	var ep extensionPair
	var err error
	ep.Name, b, err = unmarshalStringSafe(b)
	if err != nil {
		return ep, b, err
	}
	ep.Data, b, err = unmarshalStringSafe(b)
	return ep, b, err
}

type sshFxInitPacket struct {
	Version    uint32
	Extensions []extensionPair
}

func (p *sshFxInitPacket) appendTo(inB []byte) (outB []byte, err error) {
	outB = append(inB, sshFxpInit)
	outB = bigEnd_.AppendUint32(outB, p.Version)

	for _, e := range p.Extensions {
		outB = marshalString(outB, e.Name)
		outB = marshalString(outB, e.Data)
	}
	return
}

func (p *sshFxInitPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.Version, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	}
	for len(b) > 0 {
		var ep extensionPair
		ep, b, err = unmarshalExtensionPair(b)
		if err != nil {
			return err
		}
		p.Extensions = append(p.Extensions, ep)
	}
	return nil
}

type sshFxVersionPacket struct {
	Version    uint32
	Extensions []sshExtensionPair
}

type sshExtensionPair struct {
	Name, Data string
}

func (p *sshFxVersionPacket) appendTo(inB []byte) (outB []byte, err error) {
	outB = append(inB, sshFxpVersion)
	outB = bigEnd_.AppendUint32(outB, p.Version)

	for _, e := range p.Extensions {
		outB = marshalString(outB, e.Name)
		outB = marshalString(outB, e.Data)
	}
	return
}

func marshalIDStringPacket(
	packetType byte,
	id uint32,
	str string,
	inB []byte,
) (outB []byte, err error) {

	outB = append(inB, packetType)
	outB = bigEnd_.AppendUint32(outB, id)
	outB = marshalString(outB, str)
	return
}

func unmarshalIDString(b []byte, id *uint32, str *string) error {
	var err error
	*id, b, err = unmarshalUint32Safe(b)
	if err != nil {
		return err
	}
	*str, _, err = unmarshalStringSafe(b)
	return err
}

type sshFxpReaddirPacket struct {
	idPkt_
	Handle string
}

func (p *sshFxpReaddirPacket) appendTo(inB []byte) ([]byte, error) {
	return marshalIDStringPacket(sshFxpReaddir, p.ID, p.Handle, inB)
}

func (p *sshFxpReaddirPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Handle)
}

type sshFxpOpendirPacket struct {
	idPkt_
	Path string
}

func (p *sshFxpOpendirPacket) appendTo(inB []byte) ([]byte, error) {
	return marshalIDStringPacket(sshFxpOpendir, p.ID, p.Path, inB)
}

func (p *sshFxpOpendirPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Path)
}

type sshFxpLstatPacket struct {
	idPkt_
	Path string
}

func (p *sshFxpLstatPacket) appendTo(inB []byte) ([]byte, error) {
	return marshalIDStringPacket(sshFxpLstat, p.ID, p.Path, inB)
}

func (p *sshFxpLstatPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Path)
}

type sshFxpStatPacket struct {
	idPkt_
	Path string
}

func (p *sshFxpStatPacket) appendTo(inB []byte) ([]byte, error) {
	return marshalIDStringPacket(sshFxpStat, p.ID, p.Path, inB)
}

func (p *sshFxpStatPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Path)
}

type sshFxpFstatPacket struct {
	idPkt_
	Handle string
}

func (p *sshFxpFstatPacket) appendTo(inB []byte) ([]byte, error) {
	return marshalIDStringPacket(sshFxpFstat, p.ID, p.Handle, inB)
}

func (p *sshFxpFstatPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Handle)
}

type sshFxpClosePacket struct {
	idPkt_
	Handle string
}

func (p *sshFxpClosePacket) appendTo(inB []byte) ([]byte, error) {
	return marshalIDStringPacket(sshFxpClose, p.ID, p.Handle, inB)
}

func (p *sshFxpClosePacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Handle)
}

type sshFxpRemovePacket struct {
	idPkt_
	Filename string
}

func (p *sshFxpRemovePacket) appendTo(inB []byte) ([]byte, error) {
	return marshalIDStringPacket(sshFxpRemove, p.ID, p.Filename, inB)
}

func (p *sshFxpRemovePacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Filename)
}

type sshFxpRmdirPacket struct {
	idPkt_
	Path string
}

func (p *sshFxpRmdirPacket) appendTo(inB []byte) ([]byte, error) {
	return marshalIDStringPacket(sshFxpRmdir, p.ID, p.Path, inB)
}

func (p *sshFxpRmdirPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Path)
}

type sshFxpSymlinkPacket struct {
	idPkt_

	// The order of the arguments to the SSH_FXP_SYMLINK method was inadvertently reversed.
	// Unfortunately, the reversal was not noticed until the server was widely deployed.
	// Covered in Section 4.1 of https://github.com/openssh/openssh-portable/blob/master/PROTOCOL

	Targetpath string
	Linkpath   string
}

func (p *sshFxpSymlinkPacket) appendTo(inB []byte) (outB []byte, err error) {
	outB = append(inB, sshFxpSymlink)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = marshalString(outB, p.Targetpath)
	outB = marshalString(outB, p.Linkpath)
	return
}

func (p *sshFxpSymlinkPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Targetpath, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Linkpath, _, err = unmarshalStringSafe(b); err != nil {
		return err
	}
	return nil
}

type sshFxpHardlinkPacket struct {
	idPkt_
	Oldpath string
	Newpath string
}

func (p *sshFxpHardlinkPacket) appendTo(inB []byte) (outB []byte, err error) {
	const ext = "hardlink@openssh.com"

	outB = append(inB, sshFxpExtended)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = marshalString(outB, ext)
	outB = marshalString(outB, p.Oldpath)
	outB = marshalString(outB, p.Newpath)
	return
}

type sshFxpReadlinkPacket struct {
	idPkt_
	Path string
}

func (p *sshFxpReadlinkPacket) appendTo(inB []byte) ([]byte, error) {
	return marshalIDStringPacket(sshFxpReadlink, p.ID, p.Path, inB)
}

func (p *sshFxpReadlinkPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Path)
}

type sshFxpRealpathPacket struct {
	idPkt_
	Path string
}

func (p *sshFxpRealpathPacket) appendTo(inB []byte) ([]byte, error) {
	return marshalIDStringPacket(sshFxpRealpath, p.ID, p.Path, inB)
}

func (p *sshFxpRealpathPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Path)
}

//type sshFxpNameAttr struct {
//	Name     string
//	LongName string
//	Attrs    []interface{}
//}
//
//func (p *sshFxpNameAttr) appendTo(inB []byte) (outB []byte, err error) {
//	outB = marshalString(inB, p.Name)
//	outB = marshalString(outB, p.LongName)
//	for _, attr := range p.Attrs {
//		outB = marshal(outB, attr)
//	}
//	return
//}
//
//type sshFxpNamePacket struct {
//	idPkt_
//	NameAttrs []*sshFxpNameAttr
//}
//
//func (p *sshFxpNamePacket) appendTo(inB []byte) (outB []byte, err error) {
//
//	outB = append(inB, sshFxpName)
//	outB = bigEnd_.AppendUint32(outB, p.ID)
//	outB = bigEnd_.AppendUint32(outB, uint32(len(p.NameAttrs)))
//
//	for _, na := range p.NameAttrs {
//		outB, err = na.appendTo(outB)
//		if err != nil {
//			return
//		}
//	}
//	return
//}

type sshFxpOpenPacket struct {
	idPkt_
	Path   string
	Pflags uint32
	Flags  uint32
	Attrs  interface{}
}

func (p *sshFxpOpenPacket) appendTo(inB []byte) (outB []byte, err error) {

	outB = append(inB, sshFxpOpen)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = marshalString(outB, p.Path)
	outB = bigEnd_.AppendUint32(outB, p.Pflags)
	outB = bigEnd_.AppendUint32(outB, p.Flags)

	switch attrs := p.Attrs.(type) {
	case []byte:
		return // may as well short-ciruit this case.
	case os.FileInfo:
		_, fs := fileStatFromInfo(attrs) // we throw away the flags, and override with those in packet.
		return marshalFileStat(outB, p.Flags, fs), nil
	case *FileStat:
		return marshalFileStat(outB, p.Flags, attrs), nil
	}

	return marshal(outB, p.Attrs), nil
}

func (p *sshFxpOpenPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Path, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Pflags, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Flags, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	}
	p.Attrs = b
	return nil
}

//func (p *sshFxpOpenPacket) unmarshalFileStat(flags uint32) (*FileStat, error) {
//	switch attrs := p.Attrs.(type) {
//	case *FileStat:
//		return attrs, nil
//	case []byte:
//		fs, _, err := unmarshalFileStat(flags, attrs)
//		return fs, err
//	default:
//		return nil, fmt.Errorf("invalid type in unmarshalFileStat: %T", attrs)
//	}
//}

type sshFxpReadPacket struct {
	idPkt_
	Len    uint32
	Offset uint64
	Handle string
}

func (p *sshFxpReadPacket) appendTo(inB []byte) (outB []byte, err error) {
	outB = append(inB, sshFxpRead)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = marshalString(outB, p.Handle)
	outB = bigEnd_.AppendUint64(outB, p.Offset)
	outB = bigEnd_.AppendUint32(outB, p.Len)
	return
}

func (p *sshFxpReadPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Handle, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Offset, b, err = unmarshalUint64Safe(b); err != nil {
		return err
	} else if p.Len, _, err = unmarshalUint32Safe(b); err != nil {
		return err
	}
	return nil
}

// We need allocate bigger slices with extra capacity to avoid a re-allocation in sshFxpDataPacket.MarshalBinary
// So, we need: uint32(length) + byte(type) + uint32(id) + uint32(data_length)
//const dataHeaderLen = 4 + 1 + 4 + 4

/*
func (p *sshFxpReadPacket) getDataSlice(alloc *allocator, orderID uint32, maxTxPacket uint32) []byte {
	dataLen := p.Len
	if dataLen > maxTxPacket {
		dataLen = maxTxPacket
	}

	if alloc != nil {
		// GetPage returns a slice with capacity = maxMsgLength this is enough to avoid new allocations in
		// sshFxpDataPacket.MarshalBinary
		return alloc.GetPage(orderID)[:dataLen]
	}

	// allocate with extra space for the header
	return make([]byte, dataLen, dataLen+dataHeaderLen)
}
*/

type sshFxpRenamePacket struct {
	idPkt_
	Oldpath string
	Newpath string
}

func (p *sshFxpRenamePacket) appendTo(inB []byte) (outB []byte, err error) {
	outB = append(inB, sshFxpRename)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = marshalString(outB, p.Oldpath)
	outB = marshalString(outB, p.Newpath)
	return
}
func (p *sshFxpRenamePacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Oldpath, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Newpath, _, err = unmarshalStringSafe(b); err != nil {
		return err
	}
	return nil
}

type sshFxpPosixRenamePacket struct {
	idPkt_
	Oldpath string
	Newpath string
}

func (p *sshFxpPosixRenamePacket) appendTo(inB []byte) (outB []byte, err error) {
	const ext = "posix-rename@openssh.com"

	outB = append(inB, sshFxpExtended)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = marshalString(outB, ext)
	outB = marshalString(outB, p.Oldpath)
	outB = marshalString(outB, p.Newpath)
	return
}

type sshFxpWritePacket struct {
	idPkt_
	Length uint32
	Offset uint64
	Handle string
	Data   []byte // TODO - need to mark this somehow
}

func (p *sshFxpWritePacket) sizeBeforeData() int {
	// 1 (type) + 4 (id) + 4 (handle len) + len(handle) + 8 (offset) + 4 (datalen)
	return 21 + len(p.Handle)
}

func (p *sshFxpWritePacket) appendTo(inB []byte) (outB []byte, err error) {

	outB = append(inB, sshFxpWrite)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = marshalString(outB, p.Handle)
	outB = bigEnd_.AppendUint64(outB, p.Offset)
	outB = bigEnd_.AppendUint32(outB, p.Length)
	return
}

func (p *sshFxpWritePacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Handle, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Offset, b, err = unmarshalUint64Safe(b); err != nil {
		return err
	} else if p.Length, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if uint32(len(b)) < p.Length {
		return errShortPacket
	}

	p.Data = b[:p.Length]
	return nil
}

type sshFxpMkdirPacket struct {
	idPkt_
	Flags uint32 // ignored
	Path  string
}

func (p *sshFxpMkdirPacket) appendTo(inB []byte) (outB []byte, err error) {
	outB = append(inB, sshFxpMkdir)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = marshalString(outB, p.Path)
	outB = bigEnd_.AppendUint32(outB, p.Flags)
	return
}

func (p *sshFxpMkdirPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Path, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Flags, _, err = unmarshalUint32Safe(b); err != nil {
		return err
	}
	return nil
}

type sshFxpSetstatPacket struct {
	idPkt_
	Flags uint32
	Path  string
	Attrs interface{}
}

type sshFxpFsetstatPacket struct {
	idPkt_
	Flags  uint32
	Handle string
	Attrs  interface{}
}

func (p *sshFxpSetstatPacket) appendTo(inB []byte) (outB []byte, err error) {
	outB = append(inB, sshFxpSetstat)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = marshalString(outB, p.Path)
	outB = bigEnd_.AppendUint32(outB, p.Flags)

	switch attrs := p.Attrs.(type) {
	case []byte:
		return // may as well short-ciruit this case.
	case os.FileInfo:
		_, fs := fileStatFromInfo(attrs) // we throw away the flags, and override with those in packet.
		return marshalFileStat(outB, p.Flags, fs), nil
	case *FileStat:
		return marshalFileStat(outB, p.Flags, attrs), nil
	}

	return marshal(outB, p.Attrs), nil
}

func (p *sshFxpFsetstatPacket) appendTo(inB []byte) (outB []byte, err error) {
	outB = append(inB, sshFxpFsetstat)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = marshalString(outB, p.Handle)
	outB = bigEnd_.AppendUint32(outB, p.Flags)

	switch attrs := p.Attrs.(type) {
	case []byte:
		return // may as well short-ciruit this case.
	case os.FileInfo:
		_, fs := fileStatFromInfo(attrs) // we throw away the flags, and override with those in packet.
		return marshalFileStat(outB, p.Flags, fs), nil
	case *FileStat:
		return marshalFileStat(outB, p.Flags, attrs), nil
	}

	return marshal(outB, p.Attrs), nil
}

func (p *sshFxpSetstatPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Path, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Flags, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	}
	p.Attrs = b
	return nil
}

//func (p *sshFxpSetstatPacket) unmarshalFileStat(flags uint32) (*FileStat, error) {
//	switch attrs := p.Attrs.(type) {
//	case *FileStat:
//		return attrs, nil
//	case []byte:
//		fs, _, err := unmarshalFileStat(flags, attrs)
//		return fs, err
//	default:
//		return nil, fmt.Errorf("invalid type in unmarshalFileStat: %T", attrs)
//	}
//}

func (p *sshFxpFsetstatPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Handle, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Flags, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	}
	p.Attrs = b
	return nil
}

//func (p *sshFxpFsetstatPacket) unmarshalFileStat(flags uint32) (*FileStat, error) {
//	switch attrs := p.Attrs.(type) {
//	case *FileStat:
//		return attrs, nil
//	case []byte:
//		fs, _, err := unmarshalFileStat(flags, attrs)
//		return fs, err
//	default:
//		return nil, fmt.Errorf("invalid type in unmarshalFileStat: %T", attrs)
//	}
//}

//type sshFxpHandlePacket struct {
//	ID     uint32
//	Handle string
//}
//
//func (p *sshFxpHandlePacket) appendTo(inB []byte) (outB []byte, err error) {
//	outB = append(inB, sshFxpHandle)
//	outB = bigEnd_.AppendUint32(outB, p.ID)
//	outB = marshalString(outB, p.Handle)
//	return
//}
//
//type sshFxpStatusPacket struct {
//	ID uint32
//	StatusError
//}
//
//func (p *sshFxpStatusPacket) appendTo(inB []byte) (outB []byte, err error) {
//	outB = append(inB, sshFxpStatus)
//	outB = bigEnd_.AppendUint32(outB, p.ID)
//	outB = marshalStatus(outB, p.StatusError)
//	return
//}

/*
type sshFxpDataPacket struct {
	ID     uint32
	Length uint32
	Data   []byte
}

func (p *sshFxpDataPacket) marshalPacket() ([]byte, []byte, error) {
	l := 4 + 1 + 4 + // uint32(length) + byte(type) + uint32(id)
		4

	b := make([]byte, 4, l)
	b = append(b, sshFxpData)
	b = bigEnd_.AppendUint32(b, p.ID)
	b = bigEnd_.AppendUint32(b, p.Length)

	return b, p.Data, nil
}

// MarshalBinary encodes the receiver into a binary form and returns the result.
// To avoid a new allocation the Data slice must have a capacity >= Length + 9
//
// This is hand-coded rather than just append(header, payload...),
// in order to try and reuse the r.Data backing store in the packet.
func (p *sshFxpDataPacket) MarshalBinary() ([]byte, error) {
	b := append(p.Data, make([]byte, dataHeaderLen)...)
	copy(b[dataHeaderLen:], p.Data[:p.Length])
	// b[0:4] will be overwritten with the length in sendPacket
	b[4] = sshFxpData
	binary.BigEndian.PutUint32(b[5:9], p.ID)
	binary.BigEndian.PutUint32(b[9:13], p.Length)
	return b, nil
}

func (p *sshFxpDataPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Length, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if uint32(len(b)) < p.Length {
		return errShortPacket
	}

	p.Data = b[:p.Length]
	return nil
}
*/

type sshFxpStatvfsPacket struct {
	idPkt_
	Path string
}

func (p *sshFxpStatvfsPacket) appendTo(inB []byte) (outB []byte, err error) {
	const ext = "statvfs@openssh.com"
	outB = append(inB, sshFxpExtended)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = marshalString(outB, ext)
	outB = marshalString(outB, p.Path)
	return
}

// A StatVFS contains statistics about a filesystem.
type StatVFS struct {
	ID      uint32
	Bsize   uint64 // file system block size
	Frsize  uint64 // fundamental fs block size
	Blocks  uint64 // number of blocks (unit f_frsize)
	Bfree   uint64 // free blocks in file system
	Bavail  uint64 // free blocks for non-root
	Files   uint64 // total file inodes
	Ffree   uint64 // free file inodes
	Favail  uint64 // free file inodes for to non-root
	Fsid    uint64 // file system id
	Flag    uint64 // bit mask of f_flag values
	Namemax uint64 // maximum filename length
}

// TotalSpace calculates the amount of total space in a filesystem.
func (p *StatVFS) TotalSpace() uint64 {
	return p.Frsize * p.Blocks
}

// FreeSpace calculates the amount of free space in a filesystem.
func (p *StatVFS) FreeSpace() uint64 {
	return p.Frsize * p.Bfree
}

// marshalPacket converts to ssh_FXP_EXTENDED_REPLY packet binary format
func (p *StatVFS) appendTo(inB []byte) (outB []byte, err error) {
	outB = append(inB, sshFxpExtendedReply)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = bigEnd_.AppendUint64(outB, p.Bsize)
	outB = bigEnd_.AppendUint64(outB, p.Frsize)
	outB = bigEnd_.AppendUint64(outB, p.Blocks)
	outB = bigEnd_.AppendUint64(outB, p.Bfree)
	outB = bigEnd_.AppendUint64(outB, p.Bavail)
	outB = bigEnd_.AppendUint64(outB, p.Files)
	outB = bigEnd_.AppendUint64(outB, p.Ffree)
	outB = bigEnd_.AppendUint64(outB, p.Favail)
	outB = bigEnd_.AppendUint64(outB, p.Fsid)
	outB = bigEnd_.AppendUint64(outB, p.Flag)
	outB = bigEnd_.AppendUint64(outB, p.Namemax)
	return
}

type sshFxpFsyncPacket struct {
	idPkt_
	Handle string
}

func (p *sshFxpFsyncPacket) appendTo(inB []byte) (outB []byte, err error) {
	const ext = "fsync@openssh.com"

	outB = append(inB, sshFxpExtended)
	outB = bigEnd_.AppendUint32(outB, p.ID)
	outB = marshalString(outB, ext)
	outB = marshalString(outB, p.Handle)
	return
}

/*
type sshFxpExtendedPacket struct {
	ID              uint32
	ExtendedRequest string
	SpecificPacket  interface {
		serverRespondablePacket
		readonly() bool
	}
}

func (p *sshFxpExtendedPacket) id() uint32      { return p.ID }
func (p *sshFxpExtendedPacket) setId(id uint32) { p.ID = id }
func (p *sshFxpExtendedPacket) readonly() bool {
	if p.SpecificPacket == nil {
		return true
	}
	return p.SpecificPacket.readonly()
}

func (p *sshFxpExtendedPacket) respond(svr *Server) responsePacket {
	if p.SpecificPacket == nil {
		return statusFromError(p.ID, nil)
	}
	return p.SpecificPacket.respond(svr)
}

func (p *sshFxpExtendedPacket) UnmarshalBinary(b []byte) error {
	var err error
	bOrig := b
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.ExtendedRequest, _, err = unmarshalStringSafe(b); err != nil {
		return err
	}

	// specific unmarshalling
	switch p.ExtendedRequest {
	case "statvfs@openssh.com":
		p.SpecificPacket = &sshFxpExtendedPacketStatVFS{}
	case "posix-rename@openssh.com":
		p.SpecificPacket = &sshFxpExtendedPacketPosixRename{}
	case "hardlink@openssh.com":
		p.SpecificPacket = &sshFxpExtendedPacketHardlink{}
	default:
		return fmt.Errorf("packet type %v: %w", p.SpecificPacket, errUnknownExtendedPacket)
	}

	return p.SpecificPacket.UnmarshalBinary(bOrig)
}
*/

//type sshFxpExtendedPacketStatVFS struct {
//	idPkt_
//	ExtendedRequest string
//	Path            string
//}
//
//func (p *sshFxpExtendedPacketStatVFS) readonly() bool { return true }
//func (p *sshFxpExtendedPacketStatVFS) UnmarshalBinary(b []byte) error {
//	var err error
//	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
//		return err
//	} else if p.ExtendedRequest, b, err = unmarshalStringSafe(b); err != nil {
//		return err
//	} else if p.Path, _, err = unmarshalStringSafe(b); err != nil {
//		return err
//	}
//	return nil
//}
//
//type sshFxpExtendedPacketPosixRename struct {
//	idPkt_
//	ExtendedRequest string
//	Oldpath         string
//	Newpath         string
//}
//
//func (p *sshFxpExtendedPacketPosixRename) readonly() bool { return false }
//func (p *sshFxpExtendedPacketPosixRename) UnmarshalBinary(b []byte) error {
//	var err error
//	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
//		return err
//	} else if p.ExtendedRequest, b, err = unmarshalStringSafe(b); err != nil {
//		return err
//	} else if p.Oldpath, b, err = unmarshalStringSafe(b); err != nil {
//		return err
//	} else if p.Newpath, _, err = unmarshalStringSafe(b); err != nil {
//		return err
//	}
//	return nil
//}

/*
func (p *sshFxpExtendedPacketPosixRename) respond(s *Server) responsePacket {
	err := os.Rename(s.toLocalPath(p.Oldpath), s.toLocalPath(p.Newpath))
	return statusFromError(p.ID, err)
}
*/

//type sshFxpExtendedPacketHardlink struct {
//	idPkt_
//	ExtendedRequest string
//	Oldpath         string
//	Newpath         string
//}
//
//// https://github.com/openssh/openssh-portable/blob/master/PROTOCOL
//func (p *sshFxpExtendedPacketHardlink) readonly() bool { return true }
//func (p *sshFxpExtendedPacketHardlink) UnmarshalBinary(b []byte) error {
//	var err error
//	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
//		return err
//	} else if p.ExtendedRequest, b, err = unmarshalStringSafe(b); err != nil {
//		return err
//	} else if p.Oldpath, b, err = unmarshalStringSafe(b); err != nil {
//		return err
//	} else if p.Newpath, _, err = unmarshalStringSafe(b); err != nil {
//		return err
//	}
//	return nil
//}

/*
func (p *sshFxpExtendedPacketHardlink) respond(s *Server) responsePacket {
	err := os.Link(s.toLocalPath(p.Oldpath), s.toLocalPath(p.Newpath))
	return statusFromError(p.ID, err)
}
*/
