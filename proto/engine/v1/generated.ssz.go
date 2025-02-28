// Code generated by fastssz. DO NOT EDIT.
// Hash: e694201808b1e0069a274c88e1977511c2e47a735af15f5dcda9ead3d115608d
package enginev1

import (
	ssz "github.com/ferranbt/fastssz"
)

// MarshalSSZ ssz marshals the ExecutionPayload object
func (e *ExecutionPayload) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(e)
}

// MarshalSSZTo ssz marshals the ExecutionPayload object to a target array
func (e *ExecutionPayload) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf
	offset := int(508)

	// Field (0) 'ParentHash'
	if len(e.ParentHash) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, e.ParentHash...)

	// Field (1) 'FeeRecipient'
	if len(e.FeeRecipient) != 20 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, e.FeeRecipient...)

	// Field (2) 'StateRoot'
	if len(e.StateRoot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, e.StateRoot...)

	// Field (3) 'RecipientsRoot'
	if len(e.RecipientsRoot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, e.RecipientsRoot...)

	// Field (4) 'LogsBloom'
	if len(e.LogsBloom) != 256 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, e.LogsBloom...)

	// Field (5) 'Random'
	if len(e.Random) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, e.Random...)

	// Field (6) 'BlockNumber'
	dst = ssz.MarshalUint64(dst, e.BlockNumber)

	// Field (7) 'GasLimit'
	dst = ssz.MarshalUint64(dst, e.GasLimit)

	// Field (8) 'GasUsed'
	dst = ssz.MarshalUint64(dst, e.GasUsed)

	// Field (9) 'Timestamp'
	dst = ssz.MarshalUint64(dst, e.Timestamp)

	// Offset (10) 'ExtraData'
	dst = ssz.WriteOffset(dst, offset)
	offset += len(e.ExtraData)

	// Field (11) 'BaseFeePerGas'
	if len(e.BaseFeePerGas) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, e.BaseFeePerGas...)

	// Field (12) 'BlockHash'
	if len(e.BlockHash) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, e.BlockHash...)

	// Offset (13) 'Transactions'
	dst = ssz.WriteOffset(dst, offset)
	for ii := 0; ii < len(e.Transactions); ii++ {
		offset += 4
		offset += len(e.Transactions[ii])
	}

	// Field (10) 'ExtraData'
	if len(e.ExtraData) > 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, e.ExtraData...)

	// Field (13) 'Transactions'
	if len(e.Transactions) > 1048576 {
		err = ssz.ErrListTooBig
		return
	}
	{
		offset = 4 * len(e.Transactions)
		for ii := 0; ii < len(e.Transactions); ii++ {
			dst = ssz.WriteOffset(dst, offset)
			offset += len(e.Transactions[ii])
		}
	}
	for ii := 0; ii < len(e.Transactions); ii++ {
		if len(e.Transactions[ii]) > 1073741824 {
			err = ssz.ErrBytesLength
			return
		}
		dst = append(dst, e.Transactions[ii]...)
	}

	return
}

// UnmarshalSSZ ssz unmarshals the ExecutionPayload object
func (e *ExecutionPayload) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size < 508 {
		return ssz.ErrSize
	}

	tail := buf
	var o10, o13 uint64

	// Field (0) 'ParentHash'
	if cap(e.ParentHash) == 0 {
		e.ParentHash = make([]byte, 0, len(buf[0:32]))
	}
	e.ParentHash = append(e.ParentHash, buf[0:32]...)

	// Field (1) 'FeeRecipient'
	if cap(e.FeeRecipient) == 0 {
		e.FeeRecipient = make([]byte, 0, len(buf[32:52]))
	}
	e.FeeRecipient = append(e.FeeRecipient, buf[32:52]...)

	// Field (2) 'StateRoot'
	if cap(e.StateRoot) == 0 {
		e.StateRoot = make([]byte, 0, len(buf[52:84]))
	}
	e.StateRoot = append(e.StateRoot, buf[52:84]...)

	// Field (3) 'RecipientsRoot'
	if cap(e.RecipientsRoot) == 0 {
		e.RecipientsRoot = make([]byte, 0, len(buf[84:116]))
	}
	e.RecipientsRoot = append(e.RecipientsRoot, buf[84:116]...)

	// Field (4) 'LogsBloom'
	if cap(e.LogsBloom) == 0 {
		e.LogsBloom = make([]byte, 0, len(buf[116:372]))
	}
	e.LogsBloom = append(e.LogsBloom, buf[116:372]...)

	// Field (5) 'Random'
	if cap(e.Random) == 0 {
		e.Random = make([]byte, 0, len(buf[372:404]))
	}
	e.Random = append(e.Random, buf[372:404]...)

	// Field (6) 'BlockNumber'
	e.BlockNumber = ssz.UnmarshallUint64(buf[404:412])

	// Field (7) 'GasLimit'
	e.GasLimit = ssz.UnmarshallUint64(buf[412:420])

	// Field (8) 'GasUsed'
	e.GasUsed = ssz.UnmarshallUint64(buf[420:428])

	// Field (9) 'Timestamp'
	e.Timestamp = ssz.UnmarshallUint64(buf[428:436])

	// Offset (10) 'ExtraData'
	if o10 = ssz.ReadOffset(buf[436:440]); o10 > size {
		return ssz.ErrOffset
	}

	if o10 < 508 {
		return ssz.ErrInvalidVariableOffset
	}

	// Field (11) 'BaseFeePerGas'
	if cap(e.BaseFeePerGas) == 0 {
		e.BaseFeePerGas = make([]byte, 0, len(buf[440:472]))
	}
	e.BaseFeePerGas = append(e.BaseFeePerGas, buf[440:472]...)

	// Field (12) 'BlockHash'
	if cap(e.BlockHash) == 0 {
		e.BlockHash = make([]byte, 0, len(buf[472:504]))
	}
	e.BlockHash = append(e.BlockHash, buf[472:504]...)

	// Offset (13) 'Transactions'
	if o13 = ssz.ReadOffset(buf[504:508]); o13 > size || o10 > o13 {
		return ssz.ErrOffset
	}

	// Field (10) 'ExtraData'
	{
		buf = tail[o10:o13]
		if len(buf) > 32 {
			return ssz.ErrBytesLength
		}
		if cap(e.ExtraData) == 0 {
			e.ExtraData = make([]byte, 0, len(buf))
		}
		e.ExtraData = append(e.ExtraData, buf...)
	}

	// Field (13) 'Transactions'
	{
		buf = tail[o13:]
		num, err := ssz.DecodeDynamicLength(buf, 1048576)
		if err != nil {
			return err
		}
		e.Transactions = make([][]byte, num)
		err = ssz.UnmarshalDynamic(buf, num, func(indx int, buf []byte) (err error) {
			if len(buf) > 1073741824 {
				return ssz.ErrBytesLength
			}
			if cap(e.Transactions[indx]) == 0 {
				e.Transactions[indx] = make([]byte, 0, len(buf))
			}
			e.Transactions[indx] = append(e.Transactions[indx], buf...)
			return nil
		})
		if err != nil {
			return err
		}
	}
	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the ExecutionPayload object
func (e *ExecutionPayload) SizeSSZ() (size int) {
	size = 508

	// Field (10) 'ExtraData'
	size += len(e.ExtraData)

	// Field (13) 'Transactions'
	for ii := 0; ii < len(e.Transactions); ii++ {
		size += 4
		size += len(e.Transactions[ii])
	}

	return
}

// HashTreeRoot ssz hashes the ExecutionPayload object
func (e *ExecutionPayload) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(e)
}

// HashTreeRootWith ssz hashes the ExecutionPayload object with a hasher
func (e *ExecutionPayload) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'ParentHash'
	if len(e.ParentHash) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(e.ParentHash)

	// Field (1) 'FeeRecipient'
	if len(e.FeeRecipient) != 20 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(e.FeeRecipient)

	// Field (2) 'StateRoot'
	if len(e.StateRoot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(e.StateRoot)

	// Field (3) 'RecipientsRoot'
	if len(e.RecipientsRoot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(e.RecipientsRoot)

	// Field (4) 'LogsBloom'
	if len(e.LogsBloom) != 256 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(e.LogsBloom)

	// Field (5) 'Random'
	if len(e.Random) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(e.Random)

	// Field (6) 'BlockNumber'
	hh.PutUint64(e.BlockNumber)

	// Field (7) 'GasLimit'
	hh.PutUint64(e.GasLimit)

	// Field (8) 'GasUsed'
	hh.PutUint64(e.GasUsed)

	// Field (9) 'Timestamp'
	hh.PutUint64(e.Timestamp)

	// Field (10) 'ExtraData'
	{
		elemIndx := hh.Index()
		byteLen := uint64(len(e.ExtraData))
		if byteLen > 32 {
			err = ssz.ErrIncorrectListSize
			return
		}
		hh.PutBytes(e.ExtraData)
		hh.MerkleizeWithMixin(elemIndx, byteLen, (32+31)/32)
	}

	// Field (11) 'BaseFeePerGas'
	if len(e.BaseFeePerGas) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(e.BaseFeePerGas)

	// Field (12) 'BlockHash'
	if len(e.BlockHash) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(e.BlockHash)

	// Field (13) 'Transactions'
	{
		subIndx := hh.Index()
		num := uint64(len(e.Transactions))
		if num > 1048576 {
			err = ssz.ErrIncorrectListSize
			return
		}
		for _, elem := range e.Transactions {
			{
				elemIndx := hh.Index()
				byteLen := uint64(len(elem))
				if byteLen > 1073741824 {
					err = ssz.ErrIncorrectListSize
					return
				}
				hh.AppendBytes32(elem)
				hh.MerkleizeWithMixin(elemIndx, byteLen, (1073741824+31)/32)
			}
		}
		hh.MerkleizeWithMixin(subIndx, num, 1048576)
	}

	hh.Merkleize(indx)
	return
}
