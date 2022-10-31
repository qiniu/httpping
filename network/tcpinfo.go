package network

type TCPInfo struct {
	RttMs             uint32
	RttVarMs          uint32
	ReTransmitPackets uint32
	TotalPackets      uint32
	Loss              float32
}

type TCPInfoLinux struct {
	State          uint8
	Ca_state       uint8
	Retransmits    uint8
	Probes         uint8
	Backoff        uint8
	Options        uint8
	Pad_cgo_0      [2]byte
	Rto            uint32
	Ato            uint32
	Snd_mss        uint32
	Rcv_mss        uint32
	Unacked        uint32
	Sacked         uint32
	Lost           uint32
	Retrans        uint32
	Fackets        uint32
	Last_data_sent uint32
	Last_ack_sent  uint32
	Last_data_recv uint32
	Last_ack_recv  uint32
	Pmtu           uint32
	Rcv_ssthresh   uint32
	Rtt            uint32
	Rttvar         uint32
	Snd_ssthresh   uint32
	Snd_cwnd       uint32
	Advmss         uint32
	Reordering     uint32
	Rcv_rtt        uint32
	Rcv_space      uint32
	Total_retrans  uint32
}

func (t *TCPInfoLinux) common() *TCPInfo {
	var tinfo TCPInfo
	tinfo.RttMs = t.Rtt / 1000
	tinfo.RttVarMs = t.Rttvar / 1000
	tinfo.ReTransmitPackets = t.Total_retrans
	return &tinfo
}

type TCPInfoMac struct {
	Tcpi_state               uint8 /* connection state */
	Tcpi_snd_wscale          uint8 /* Window scale for send window */
	Tcpi_rcv_wscale          uint8 /* Window scale for receive window */
	__pad1                   uint8
	Tcpi_options             uint32 /* TCP options supported */
	Tcpi_flags               uint32 /* flags */
	Tcpi_rto                 uint32 /* retransmit timeout in ms */
	Tcpi_maxseg              uint32 /* maximum segment size supported */
	Tcpi_snd_ssthresh        uint32 /* slow start threshold in bytes */
	Tcpi_snd_cwnd            uint32 /* send congestion window in bytes */
	Tcpi_snd_wnd             uint32 /* send widnow in bytes */
	Tcpi_snd_sbbytes         uint32 /* bytes in send socket buffer, including in-flight data */
	Tcpi_rcv_wnd             uint32 /* receive window in bytes*/
	Tcpi_rttcur              uint32 /* most recent RTT in ms */
	Tcpi_srtt                uint32 /* average RTT in ms */
	Tcpi_rttvar              uint32 /* RTT variance */
	Tcpi__tfo                uint32
	Tcpi_txpackets           uint64
	Tcpi_txbytes             uint64
	Tcpi_txretransmitbytes   uint64
	Tcpi_rxpackets           uint64
	Tcpi_rxbytes             uint64
	Tcpi_rxoutoforderbytes   uint64
	Tcpi_txretransmitpackets uint64
}

func (t *TCPInfoMac) common() *TCPInfo {
	var tinfo TCPInfo
	tinfo.ReTransmitPackets = uint32(t.Tcpi_txretransmitpackets)
	tinfo.RttMs = t.Tcpi_srtt
	tinfo.RttVarMs = t.Tcpi_rttvar
	tinfo.TotalPackets = uint32(t.Tcpi_txpackets)
	return &tinfo
}
