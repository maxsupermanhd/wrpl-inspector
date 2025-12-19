package wrpl

// func WriteReplay(header WRPLHeader, settings, packets, results []byte) ([]byte, error) {
// 	// header.ResultsBlkOffset
// 	header.SettingsBLKSize = uint16(len(settings))

// 	buf := &bytes.Buffer{}
// 	err := binary.Write(buf, binary.LittleEndian, header)
// 	if err != nil {
// 		return nil, err
// 	}
// 	pkw, err := zlib.NewWriterLevel(buf, 3)
// 	if err != nil {
// 		return nil, err
// 	}
// 	_, err = pkw.Write(packets)
// 	if err != nil {
// 		return nil, err
// 	}
// 	pkw.Close()
// 	rpl.Header.ResultsBlkOffset = int32(buf.Len())
// 	buf2 := &bytes.Buffer{}
// 	err = binary.Write(buf2, binary.LittleEndian, rpl.Header)
// 	if err != nil {
// 		return nil, err
// 	}
// 	ret := buf.Bytes()
// 	ret2 := buf2.Bytes()
// 	for i := range len(ret2) {
// 		ret[i] = ret2[i]
// 	}
// 	_, err = buf.Write(rpl.ResultsBLK)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return ret, nil
// }
