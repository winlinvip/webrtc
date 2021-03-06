package samplebuilder

import (
	"fmt"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
)

type sampleBuilderTest struct {
	message         string
	packets         []*rtp.Packet
	withHeadChecker bool
	headBytes       []byte
	samples         []*media.Sample
	timestamps      []uint32
	maxLate         uint16
}

type fakeDepacketizer struct {
}

func (f *fakeDepacketizer) Unmarshal(r []byte) ([]byte, error) {
	return r, nil
}

type fakePartitionHeadChecker struct {
	headBytes []byte
}

func (f *fakePartitionHeadChecker) IsPartitionHead(payload []byte) bool {
	for _, b := range f.headBytes {
		if payload[0] == b {
			return true
		}
	}
	return false
}

func TestSampleBuilder(t *testing.T) {
	testData := []sampleBuilderTest{
		{
			message: "SampleBuilder shouldn't emit anything if only one RTP packet has been pushed",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
			},
			samples:    []*media.Sample{},
			timestamps: []uint32{},
			maxLate:    50,
		},
		{
			message: "SampleBuilder should emit one packet, we had three packets with unique timestamps",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 7}, Payload: []byte{0x03}},
			},
			samples: []*media.Sample{
				{Data: []byte{0x02}, Duration: time.Second},
			},
			timestamps: []uint32{
				6,
			},
			maxLate: 50,
		},
		{
			message: "SampleBuilder should emit one packet, we had two packets but two with duplicate timestamps",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 6}, Payload: []byte{0x03}},
				{Header: rtp.Header{SequenceNumber: 5003, Timestamp: 7}, Payload: []byte{0x04}},
			},
			samples: []*media.Sample{
				{Data: []byte{0x02, 0x03}, Duration: time.Second},
			},
			timestamps: []uint32{
				6,
			},
			maxLate: 50,
		},
		{
			message: "SampleBuilder shouldn't emit a packet because we have a gap before a valid one",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 7}, Payload: []byte{0x03}},
			},
			samples:    []*media.Sample{},
			timestamps: []uint32{},
			maxLate:    50,
		},
		{
			message: "SampleBuilder should emit a packet after a gap if PartitionHeadChecker assumes it head",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 7}, Payload: []byte{0x03}},
			},
			withHeadChecker: true,
			headBytes:       []byte{0x02},
			samples: []*media.Sample{
				{Data: []byte{0x02}, Duration: 0},
			},
			timestamps: []uint32{
				6,
			},
			maxLate: 50,
		},
		{
			message: "SampleBuilder shouldn't emit a packet after a gap if PartitionHeadChecker doesn't assume it head",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 7}, Payload: []byte{0x03}},
			},
			withHeadChecker: true,
			headBytes:       []byte{},
			samples:         []*media.Sample{},
			timestamps:      []uint32{},
			maxLate:         50,
		},
		{
			message: "SampleBuilder should emit multiple valid packets",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 1}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 2}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 3}, Payload: []byte{0x03}},
				{Header: rtp.Header{SequenceNumber: 5003, Timestamp: 4}, Payload: []byte{0x04}},
				{Header: rtp.Header{SequenceNumber: 5004, Timestamp: 5}, Payload: []byte{0x05}},
				{Header: rtp.Header{SequenceNumber: 5005, Timestamp: 6}, Payload: []byte{0x06}},
			},
			samples: []*media.Sample{
				{Data: []byte{0x02}, Duration: time.Second},
				{Data: []byte{0x03}, Duration: time.Second},
				{Data: []byte{0x04}, Duration: time.Second},
				{Data: []byte{0x05}, Duration: time.Second},
			},
			timestamps: []uint32{
				2,
				3,
				4,
				5,
			},
			maxLate: 50,
		},
	}

	t.Run("Pop", func(t *testing.T) {
		assert := assert.New(t)

		for _, t := range testData {
			var opts []Option
			if t.withHeadChecker {
				opts = append(opts, WithPartitionHeadChecker(
					&fakePartitionHeadChecker{headBytes: t.headBytes},
				))
			}

			s := New(t.maxLate, &fakeDepacketizer{}, 1, opts...)
			samples := []*media.Sample{}

			for _, p := range t.packets {
				s.Push(p)
			}
			for sample := s.Pop(); sample != nil; sample = s.Pop() {
				samples = append(samples, sample)
			}

			assert.Equal(samples, t.samples, t.message)
		}
	})
	t.Run("PopWithTimestamp", func(t *testing.T) {
		assert := assert.New(t)

		for _, t := range testData {
			var opts []Option
			if t.withHeadChecker {
				opts = append(opts, WithPartitionHeadChecker(
					&fakePartitionHeadChecker{headBytes: t.headBytes},
				))
			}

			s := New(t.maxLate, &fakeDepacketizer{}, 1, opts...)
			samples := []*media.Sample{}
			timestamps := []uint32{}

			for _, p := range t.packets {
				s.Push(p)
			}
			for sample, timestamp := s.PopWithTimestamp(); sample != nil; sample, timestamp = s.PopWithTimestamp() {
				samples = append(samples, sample)
				timestamps = append(timestamps, timestamp)
			}

			assert.Equal(samples, t.samples, t.message)
			assert.Equal(timestamps, t.timestamps, t.message)
		}
	})
}

// SampleBuilder should respect maxLate if we popped successfully but then have a gap larger then maxLate
func TestSampleBuilderMaxLate(t *testing.T) {
	assert := assert.New(t)
	s := New(50, &fakeDepacketizer{}, 1)

	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 0, Timestamp: 1}, Payload: []byte{0x01}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 1, Timestamp: 2}, Payload: []byte{0x01}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 2, Timestamp: 3}, Payload: []byte{0x01}})
	assert.Equal(s.Pop(), &media.Sample{Data: []byte{0x01}, Duration: time.Second}, "Failed to build samples before gap")

	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 500}, Payload: []byte{0x02}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 501}, Payload: []byte{0x02}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 502}, Payload: []byte{0x02}})
	assert.Equal(s.Pop(), &media.Sample{Data: []byte{0x02}, Duration: time.Second}, "Failed to build samples after large gap")
}

func TestSeqnumDistance(t *testing.T) {
	testData := []struct {
		x uint16
		y uint16
		d uint16
	}{
		{0x0001, 0x0003, 0x0002},
		{0x0003, 0x0001, 0x0002},
		{0xFFF3, 0xFFF1, 0x0002},
		{0xFFF1, 0xFFF3, 0x0002},
		{0xFFFF, 0x0001, 0x0002},
		{0x0001, 0xFFFF, 0x0002},
	}

	for _, data := range testData {
		if ret := seqnumDistance(data.x, data.y); ret != data.d {
			t.Errorf("seqnumDistance(%d, %d) returned %d which must be %d",
				data.x, data.y, ret, data.d)
		}
	}
}

func TestSampleBuilderCleanReference(t *testing.T) {
	for _, seqStart := range []uint16{
		0,
		0xFFF8, // check upper boundary
		0xFFFE, // check upper boundary
	} {
		seqStart := seqStart
		t.Run(fmt.Sprintf("From%d", seqStart), func(t *testing.T) {
			s := New(10, &fakeDepacketizer{}, 1)

			s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 0 + seqStart, Timestamp: 0}, Payload: []byte{0x01}})
			s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 1 + seqStart, Timestamp: 0}, Payload: []byte{0x02}})
			s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 2 + seqStart, Timestamp: 0}, Payload: []byte{0x03}})
			pkt4 := &rtp.Packet{Header: rtp.Header{SequenceNumber: 14 + seqStart, Timestamp: 120}, Payload: []byte{0x04}}
			s.Push(pkt4)
			pkt5 := &rtp.Packet{Header: rtp.Header{SequenceNumber: 12 + seqStart, Timestamp: 120}, Payload: []byte{0x05}}
			s.Push(pkt5)

			for i := 0; i < 3; i++ {
				if s.buffer[(i+int(seqStart))%0x10000] != nil {
					t.Errorf("Old packet (%d) is not unreferenced (maxLate: 10, pushed: 12)", i)
				}
			}
			if s.buffer[(14+int(seqStart))%0x10000] != pkt4 {
				t.Error("New packet must be referenced after jump")
			}
			if s.buffer[(12+int(seqStart))%0x10000] != pkt5 {
				t.Error("New packet must be referenced after jump")
			}
		})
	}
}

func TestSampleBuilderWithPacketReleaseHandler(t *testing.T) {
	var released []*rtp.Packet
	fakePacketReleaseHandler := func(p *rtp.Packet) {
		released = append(released, p)
	}

	// Test packets released via 'maxLate'.
	pkts := []rtp.Packet{
		{Header: rtp.Header{SequenceNumber: 0, Timestamp: 0}, Payload: []byte{0x01}},
		{Header: rtp.Header{SequenceNumber: 11, Timestamp: 120}, Payload: []byte{0x02}},
		{Header: rtp.Header{SequenceNumber: 12, Timestamp: 121}, Payload: []byte{0x03}},
		{Header: rtp.Header{SequenceNumber: 13, Timestamp: 122}, Payload: []byte{0x04}},
	}
	s := New(10, &fakeDepacketizer{}, 1, WithPacketReleaseHandler(fakePacketReleaseHandler))
	s.Push(&pkts[0])
	s.Push(&pkts[1])
	if len(released) == 0 {
		t.Errorf("Old packet is not released")
	}
	if len(released) > 0 && released[0].SequenceNumber != pkts[0].SequenceNumber {
		t.Errorf("Unexpected packet released by maxLate")
	}
	// Test packets released after samples built.
	s.Push(&pkts[2])
	s.Push(&pkts[3])
	if s.Pop() == nil {
		t.Errorf("Should have some sample here.")
	}
	if len(released) != 2 {
		t.Errorf("packet built with sample is not released")
	}
	if len(released) >= 2 && released[1].SequenceNumber != pkts[2].SequenceNumber {
		t.Errorf("Unexpected packet released by samples built")
	}
}
