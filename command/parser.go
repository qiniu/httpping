// from https://github.com/sggms/go-pingparse
package command

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotEnoughLines       = errors.New("not enough lines")
	ErrHeaderMismatch       = errors.New("header mismatch")
	ErrUnknownHost          = errors.New("unkown host")
	ErrUnrecognizedLine     = errors.New("unrecognized ping reply line")
	ErrMalformedStatsHeader = errors.New("malformed stats header")
	ErrMalformedStatsLine1  = errors.New("malformed stats line 1")
	ErrMalformedStatsLine2  = errors.New("malformed stats line 2")
)

type ConversionError struct {
	Context string
	Err     error
}

func (ce ConversionError) Error() string {
	return fmt.Sprintf("%s: %v", ce.Context, ce.Err)
}

var (
	headerRx         = regexp.MustCompile(`^PING (?P<host>.*) \((?P<resolvedIPAddress>\d+\.\d+\.\d+\.\d+)\)( from .* :)? (?P<payloadSize>\d+)\((?P<payloadActualSize>\d+)\) bytes of data`)
	headerRxAlt      = regexp.MustCompile(`^PING (?P<host>.*) \((?P<resolvedIPAddress>\d+\.\d+\.\d+\.\d+)\)( from .*)?: (?P<payloadSize>\d+) data bytes`)
	lineRx           = regexp.MustCompile(`^(?P<replySize>\d+) bytes from (?P<fromAddress>\d+\.\d+\.\d+\.\d+): icmp_seq=(?P<seqNo>\d+) ttl=(?P<ttl>\d+) time=(?P<time>.*)$`)
	statsSeparatorRx = regexp.MustCompile(`^--- (?P<IPAddress>.*) ping statistics ---$`)
	statsLine1       = regexp.MustCompile(`^(?P<packetsTransmitted>\d+) packets transmitted, (?P<packetsReceived>\d+) (packets )?received,( \+(?P<errors>\d+) errors,)?( \+(?P<duplicates>\d+) duplicates,)?( (?P<packetLoss>.*)% packet loss)?(, time (?P<time>.*))?( \-\- (?P<warning>.*))?$`)
	statsLine2       = regexp.MustCompile(`^(rtt|round-trip) min/avg/max/(mdev|stddev) = (?P<min>[^/]+)/(?P<avg>[^/]+)/(?P<max>[^/]+)/(?P<mdev>[^ ]+) (?P<unit>.*)$`)
	pipeNo           = regexp.MustCompile(`(?P<unit>[^,]+), pipe (?P<pipeNo>\d+)$`)
	pipeNoLine       = regexp.MustCompile(`^pipe (?P<pipeNo>\d+)$`)
	hostErrorLineRx1 = regexp.MustCompile(`^From (?P<fromAddress>\d+\.\d+\.\d+\.\d+) icmp_seq=(?P<seqNo>\d+) (?P<error>.*)$`)
	hostErrorLineRx2 = regexp.MustCompile(`^(?P<replySize>\d+) bytes from (?P<fromAddress>\d+\.\d+\.\d+\.\d+): (?P<error>.*)$`)
)

// PingOutput contains the whole ping operation output.
type PingOutput struct {
	Host              string
	ResolvedIPAddress string
	PayloadSize       uint
	PayloadActualSize uint
	Replies           []PingReply
	Stats             PingStatistics
}

// PingReply contains an individual ping reply line.
type PingReply struct {
	Size           uint
	FromAddress    string
	SequenceNumber uint
	TTL            uint
	Time           time.Duration
	Error          string
	Duplicate      bool
}

// PingStatistics contains the statistics of the whole ping operation.
type PingStatistics struct {
	IPAddress          string
	PacketsTransmitted uint
	PacketsReceived    uint
	Errors             uint
	PacketLossPercent  float32
	Time               time.Duration
	RoundTripMin       time.Duration
	RoundTripAverage   time.Duration
	RoundTripMax       time.Duration
	RoundTripDeviation time.Duration
	Warning            string
}

func matchAsMap(rx *regexp.Regexp, s string) map[string]string {
	m := rx.FindStringSubmatch(s)
	result := make(map[string]string)
	if len(m) != 0 {
		for i, name := range rx.SubexpNames()[1:] {
			result[name] = m[i+1]
		}
	}

	return result
}

// Parse will parse the specified ping output and return all the information in a a PingOutput object.
func Parse(s string) (*PingOutput, error) {
	var po PingOutput

	// separate full output text into lines
	lines := strings.Split(s, "\n")
	if len(lines) < 4 {
		if len(lines) != 0 {
			if lines[0] == "ping: unknown host" {
				return nil, ErrUnknownHost
			}
		}
		return nil, ErrNotEnoughLines
	}
	var result map[string]string
	if runtime.GOOS == "darwin" {
		result = matchAsMap(headerRxAlt, lines[0])
	} else {
		result = matchAsMap(headerRx, lines[0])
		if len(result) == 0 {
			result = matchAsMap(headerRxAlt, lines[0])
			if len(result) == 0 {
				return nil, ErrHeaderMismatch
			}
		}
	}

	po.Host = result["host"]
	po.ResolvedIPAddress = result["resolvedIPAddress"]
	payloadSize, err := strconv.ParseUint(result["payloadSize"], 10, 64)
	if err != nil {
		return nil, ConversionError{"payloadSize", err}
	}
	po.PayloadSize = uint(payloadSize)

	if v, ok := result["payloadActualSize"]; ok {
		payloadActualSize, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, ConversionError{"payloadActualSize", err}
		}
		po.PayloadActualSize = uint(payloadActualSize)
	}

	// start parsing replies
	var last int
	for i, line := range lines[1:] {
		if line == "" {
			last = i + 2
			break
		}
		var pr PingReply

		// remove DUP postfix (if any)
		if strings.HasSuffix(line, " (DUP!)") {
			pr.Duplicate = true
			line = line[:len(line)-7]
		}

		result = matchAsMap(lineRx, line)
		if len(result) == 0 {
			// try to match a host error line
			result = matchAsMap(hostErrorLineRx1, line)
			if len(result) == 0 {
				result = matchAsMap(hostErrorLineRx2, line)
				if len(result) == 0 {
					// some ping outputs have a new line separator, others don't
					result = matchAsMap(statsSeparatorRx, line)
					if len(result) != 0 {
						last = i + 1
						break
					}

					return nil, ErrUnrecognizedLine
				}
			}
		}

		if v, ok := result["replySize"]; ok && len(v) != 0 {
			replySize, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return nil, ConversionError{"replySize", err}
			}
			pr.Size = uint(replySize)
		}
		pr.FromAddress = result["fromAddress"]
		pr.Error = result["error"]

		if v, ok := result["seqNo"]; ok && len(v) != 0 {
			replySeqNo, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return nil, ConversionError{"reply seqNo", err}
			}
			pr.SequenceNumber = uint(replySeqNo)
		}

		if v, ok := result["ttl"]; ok && len(v) != 0 {
			replyTTL, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return nil, ConversionError{"ttl", err}
			}
			pr.TTL = uint(replyTTL)
		}

		if v, ok := result["time"]; ok && len(v) != 0 {
			pr.Time, err = time.ParseDuration(strings.Replace(v, " ", "", -1))
			if err != nil {
				return nil, ConversionError{"ping reply time", err}
			}
		}

		// cleanup 'result', as it is checked after the loop
		result = map[string]string{}

		po.Replies = append(po.Replies, pr)
	}

	if len(result) == 0 {
		if last >= len(lines) {
			return nil, ErrNotEnoughLines
		}
		// parse header
		result = matchAsMap(statsSeparatorRx, lines[last])
		if len(result) == 0 {
			return nil, ErrMalformedStatsHeader
		}
	}
	po.Stats.IPAddress = result["IPAddress"]

	// parse stats line 1
	last++
	result = matchAsMap(statsLine1, lines[last])
	if len(result) == 0 {
		return nil, ErrMalformedStatsLine1
	}
	packetsTransmitted, err := strconv.ParseUint(result["packetsTransmitted"], 10, 64)
	if err != nil {
		return nil, ConversionError{"packetsTransmitted", err}
	}
	po.Stats.PacketsTransmitted = uint(packetsTransmitted)

	// a negative packets received count will trigger a conversion error here
	packetsReceived, err := strconv.ParseUint(result["packetsReceived"], 10, 64)
	if err != nil {
		return nil, ConversionError{"packetsReceived", err}
	}
	po.Stats.PacketsReceived = uint(packetsReceived)

	if v, ok := result["errors"]; ok && len(v) != 0 {
		errCount, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, ConversionError{"stats errors", err}
		}
		po.Stats.Errors = uint(errCount)
	}

	if v, ok := result["packetLoss"]; ok && len(v) != 0 {
		packetLossPcent, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, ConversionError{"packetLoss", err}
		}
		po.Stats.PacketLossPercent = float32(packetLossPcent)
	} else {
		po.Stats.Warning = result["warning"]
	}

	if v, ok := result["time"]; ok && len(v) != 0 {
		po.Stats.Time, err = time.ParseDuration(v)
		if err != nil {
			return nil, ConversionError{"stats time", err}
		}
	}

	validReplies := 0
	// check if a summary second line of stats is expected
	for _, pr := range po.Replies {
		if pr.Error != `` {
			continue
		}
		validReplies++
	}
	if validReplies == 0 {
		return &po, nil
	}

	// parse stats line 2
	last++
	result = matchAsMap(statsLine2, lines[last])
	if len(result) == 0 {
		result = matchAsMap(pipeNoLine, lines[last])
		if len(result) != 0 {
			// ignore pipe number
			return &po, nil
		}

		log.Println("faulty:", lines[last])

		return nil, ErrMalformedStatsLine2
	}

	unit := result["unit"]
	pm := matchAsMap(pipeNo, unit)
	if len(pm) != 0 {
		unit = pm["unit"]
		// pipe number is ignored
	}

	po.Stats.RoundTripMin, err = time.ParseDuration(result["min"] + unit)
	if err != nil {
		return nil, ConversionError{"rtt", err}
	}
	po.Stats.RoundTripAverage, err = time.ParseDuration(result["avg"] + unit)
	if err != nil {
		return nil, ConversionError{"avg", err}
	}
	po.Stats.RoundTripMax, err = time.ParseDuration(result["max"] + unit)
	if err != nil {
		return nil, ConversionError{"max", err}
	}
	po.Stats.RoundTripDeviation, err = time.ParseDuration(result["mdev"] + unit)
	if err != nil {
		return nil, ConversionError{"mdev", err}
	}

	return &po, nil
}
