package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/go-mp3"
)

type DecodedAudio struct {
	SampleRate int
	Channels   int
	PCM        []byte // Signed 16-bit Little Endian stereo PCM
}

func LoadAudio(filePath string) (*DecodedAudio, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3":
		return loadMP3(filePath)
	case ".wav":
		return loadWAV(filePath)
	default:
		// Attempt MP3 then WAV
		a, err := loadMP3(filePath)
		if err == nil {
			return a, nil
		}
		return loadWAV(filePath)
	}
}

func loadMP3(filePath string) (*DecodedAudio, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, fmt.Errorf("mp3 decode error for %s: %w", filepath.Base(filePath), err)
	}

	raw, err := io.ReadAll(dec)
	if err != nil {
		return nil, fmt.Errorf("mp3 read error for %s: %w", filepath.Base(filePath), err)
	}

	return &DecodedAudio{
		SampleRate: dec.SampleRate(),
		Channels:   2,
		PCM:        raw,
	}, nil
}

func loadWAV(filePath string) (*DecodedAudio, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if len(data) < 44 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, fmt.Errorf("invalid wav header: %s", filepath.Base(filePath))
	}

	// Simple parser for RIFF WAVE PCM chunks
	idx := 12
	var audioFormat, numChannels uint16
	var sampleRate uint32
	var bitsPerSample uint16
	var pcmData []byte

	for idx+8 <= len(data) {
		chunkID := string(data[idx : idx+4])
		chunkSize := binary.LittleEndian.Uint32(data[idx+4 : idx+8])
		chunkDataStart := idx + 8
		chunkDataEnd := chunkDataStart + int(chunkSize)
		if chunkDataEnd > len(data) {
			chunkDataEnd = len(data)
		}

		if chunkID == "fmt " && chunkSize >= 16 {
			audioFormat = binary.LittleEndian.Uint16(data[chunkDataStart : chunkDataStart+2])
			numChannels = binary.LittleEndian.Uint16(data[chunkDataStart+2 : chunkDataStart+4])
			sampleRate = binary.LittleEndian.Uint32(data[chunkDataStart+4 : chunkDataStart+8])
			bitsPerSample = binary.LittleEndian.Uint16(data[chunkDataStart+14 : chunkDataStart+16])
		} else if chunkID == "data" {
			pcmData = data[chunkDataStart:chunkDataEnd]
		}

		idx = chunkDataEnd
		// Pad byte if chunk size is odd
		if chunkSize%2 != 0 {
			idx++
		}
	}

	if audioFormat != 1 { // 1 = PCM
		return nil, fmt.Errorf("unsupported wav format (%d), only uncompressed PCM is supported", audioFormat)
	}
	if bitsPerSample != 16 {
		return nil, fmt.Errorf("unsupported wav bit depth (%d), only 16-bit PCM is supported", bitsPerSample)
	}

	// If mono, convert to stereo
	if numChannels == 1 {
		stereo := make([]byte, len(pcmData)*2)
		for i := 0; i < len(pcmData); i += 2 {
			stereo[i*2] = pcmData[i]
			stereo[i*2+1] = pcmData[i+1]
			stereo[i*2+2] = pcmData[i]
			stereo[i*2+3] = pcmData[i+1]
		}
		pcmData = stereo
		numChannels = 2
	}

	return &DecodedAudio{
		SampleRate: int(sampleRate),
		Channels:   int(numChannels),
		PCM:        pcmData,
	}, nil
}

// ApplyVolume scales 16-bit PCM samples with volume (0.0 to 1.0) and soft saturation
func ApplyVolume(pcm []byte, volume float64) []byte {
	if volume >= 0.999 {
		out := make([]byte, len(pcm))
		copy(out, pcm)
		return out
	}
	if volume <= 0.001 {
		return make([]byte, len(pcm))
	}

	out := make([]byte, len(pcm))
	for i := 0; i < len(pcm); i += 2 {
		sample := int16(binary.LittleEndian.Uint16(pcm[i : i+2]))
		scaled := float64(sample) * volume
		if scaled > 32767 {
			scaled = 32767
		} else if scaled < -32768 {
			scaled = -32768
		}
		binary.LittleEndian.PutUint16(out[i:i+2], uint16(int16(scaled)))
	}
	return out
}

func GenerateSilence(durationSeconds float64, sampleRate int) []byte {
	numBytes := int(float64(sampleRate*4) * durationSeconds)
	return make([]byte, numBytes)
}

// ResampleTo44100 resamples any sample rate to 44100Hz stereo 16-bit PCM using linear interpolation
func ResampleTo44100(pcm []byte, fromSampleRate int) []byte {
	if fromSampleRate == 44100 {
		return pcm
	}
	numInputFrames := len(pcm) / 4 // 2 channels * 2 bytes = 4 bytes per frame
	ratio := 44100.0 / float64(fromSampleRate)
	numOutputFrames := int(float64(numInputFrames) * ratio)
	out := make([]byte, numOutputFrames*4)

	for i := 0; i < numOutputFrames; i++ {
		origPos := float64(i) / ratio
		idx0 := int(origPos)
		idx1 := idx0 + 1
		frac := origPos - float64(idx0)

		if idx1 >= numInputFrames {
			idx1 = numInputFrames - 1
		}
		if idx0 >= numInputFrames {
			idx0 = numInputFrames - 1
		}

		// Channel Left
		left0 := int16(binary.LittleEndian.Uint16(pcm[idx0*4 : idx0*4+2]))
		left1 := int16(binary.LittleEndian.Uint16(pcm[idx1*4 : idx1*4+2]))
		leftVal := int16(float64(left0)*(1.0-frac) + float64(left1)*frac)

		// Channel Right
		right0 := int16(binary.LittleEndian.Uint16(pcm[idx0*4+2 : idx0*4+4]))
		right1 := int16(binary.LittleEndian.Uint16(pcm[idx1*4+2 : idx1*4+4]))
		rightVal := int16(float64(right0)*(1.0-frac) + float64(right1)*frac)

		binary.LittleEndian.PutUint16(out[i*4:i*4+2], uint16(leftVal))
		binary.LittleEndian.PutUint16(out[i*4+2:i*4+4], uint16(rightVal))
	}

	return out
}

// CreateAudioStream converts decoded audio into standardized 44100Hz stereo PCM
func PrepareAudio(decoded *DecodedAudio, volume float64) []byte {
	pcm := decoded.PCM
	if decoded.SampleRate != 44100 {
		pcm = ResampleTo44100(pcm, decoded.SampleRate)
	}
	return ApplyVolume(pcm, volume)
}

type AudioBufferReader struct {
	reader *bytes.Reader
}

func NewAudioBufferReader(data []byte) *AudioBufferReader {
	return &AudioBufferReader{reader: bytes.NewReader(data)}
}

func (r *AudioBufferReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}
