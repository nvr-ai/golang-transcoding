package main

import (
	"bytes"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/deepch/vdk/format/rtspv2"
	"github.com/deepch/vdk/format/ts"
	"log"
	"os"
	"time"
)

type Segment struct {
	IDd      string        `json:"id"`
	Filename string        `json:"filename"`
	Date     time.Time     `json:"date"`
	Duration time.Duration `json:"duration"`
}

func main() {

	annexbNALUStartCode := func() []byte { return []byte{0x00, 0x00, 0x00, 0x01} }

	client, _ := rtspv2.Dial(rtspv2.RTSPClientOptions{URL: "rtsp://admin:HIKv123001!@bassett-rtsp.streamnvr.com:10800", DisableAudio: false, DialTimeout: 3 * time.Second, ReadWriteTimeout: 3 * time.Second, Debug: false})

	var last *av.Packet

	cur := make([]*av.Packet, 0)
	count := 0
	i := 0

	for pkt := range client.OutgoingPacketQueue {
		pkt.Data = pkt.Data[4:]
		i++

		//
		// If the packet is a key-frame, then we need to save the previous batch of packets to disk
		// and start a new batch.
		//
		if pkt.IsKeyFrame {
			count++
			// For every key-frame pre-pend the SPS and PPS
			if pkt.IsKeyFrame {
				pkt.Data = append(annexbNALUStartCode(), pkt.Data...)
				pkt.Data = append(client.CodecData[0].(h264parser.CodecData).PPS(), pkt.Data...)
				pkt.Data = append(annexbNALUStartCode(), pkt.Data...)
				pkt.Data = append(client.CodecData[0].(h264parser.CodecData).SPS(), pkt.Data...)
				pkt.Data = append(annexbNALUStartCode(), pkt.Data...)
			}
			last = pkt
		}

		if count == 1 {
			println("Saving batch to disk")

			cur = append(cur, last)

			go process(cur, client.CodecData)

			//
			// ** This is where I want to change the mp4 video and make it smaller, etc.. **
			//

			cur = nil
			cur = append(cur, last)
			count = 0
		} else {
			//
			// Accumulating packets for the next batch to save to a new .ts (mp4) file..
			//
			cur = append(cur, pkt)
		}
	}
}

func process(pkts []*av.Packet, codecs []av.CodecData) {
	filename := time.Now().Format("2006-01-02-15-04-05")

	f, _ := os.Create(filename + ".ts")
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			panic(err)
		}
	}(f)

	outfile := bytes.NewBuffer([]byte{})
	Muxer := ts.NewMuxer(outfile)
	err := Muxer.WriteHeader(codecs)
	if err != nil {
		println(err.Error())
		return
	}
	Muxer.PaddingToMakeCounterCont = true

	for _, v := range pkts {
		v.CompositionTime = 1
		err := Muxer.WritePacket(*v)
		if err != nil {
			log.Println(err)
			return
		}
	}
	err = Muxer.WriteTrailer()
	if err != nil {
		log.Println(err)
		return
	}

	_, err = f.Write(outfile.Bytes())
	if err != nil {
		println(err.Error())
	}
}
