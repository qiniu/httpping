package network

import (
	"errors"
	"syscall"
)

type TCPInfo struct {
	RttMs             uint32
	RttVarMs          uint32
	ReTransmitPackets uint32
	TotalPackets      uint32
}

// go struct for low version linux kernel
//type TCPInfoLinux struct {
//	State          uint8
//	Ca_state       uint8
//	Retransmits    uint8
//	Probes         uint8
//	Backoff        uint8
//	Options        uint8
//	Pad_cgo_0      [2]byte
//	Rto            uint32
//	Ato            uint32
//	Snd_mss        uint32
//	Rcv_mss        uint32
//	Unacked        uint32
//	Sacked         uint32
//	Lost           uint32
//	Retrans        uint32
//	Fackets        uint32
//	Last_data_sent uint32
//	Last_ack_sent  uint32
//	Last_data_recv uint32
//	Last_ack_recv  uint32
//	Pmtu           uint32
//	Rcv_ssthresh   uint32
//	Rtt            uint32
//	Rttvar         uint32
//	Snd_ssthresh   uint32
//	Snd_cwnd       uint32
//	Advmss         uint32
//	Reordering     uint32
//	Rcv_rtt        uint32
//	Rcv_space      uint32
//	Total_retrans  uint32
//}

// include/uapi/linux/tcp.h
type TCPInfoLinux struct {

	/*__u8*/ Tcpi_state uint8
	/*__u8*/ Tcpi_ca_state uint8
	/*__u8*/ Tcpi_retransmits uint8
	/*__u8*/ Tcpi_probes uint8
	/*__u8*/ Tcpi_backoff uint8
	/*__u8*/ Tcpi_options uint8
	/*__u8*/ Tcpi_snd_recv_wscale uint8 // Tcpi_snd_wscale : 4, Tcpi_rcv_wscale : 4
	/*__u8*/ reserved uint8 //Tcpi_delivery_rate_app_limited:1, Tcpi_fastopen_client_fail:2

	/*__u32*/
	Tcpi_rto uint32
	/*__u32*/ Tcpi_ato uint32
	/*__u32*/ Tcpi_snd_mss uint32
	/*__u32*/ Tcpi_rcv_mss uint32

	/*__u32*/
	Tcpi_unacked uint32
	/*__u32*/ Tcpi_sacked uint32
	/*__u32*/ Tcpi_lost uint32
	/*__u32*/ Tcpi_retrans uint32
	/*__u32*/ Tcpi_fackets uint32

	/* Times. */
	/*__u32*/
	Tcpi_last_data_sent uint32
	/*__u32*/ Tcpi_last_ack_sent uint32 /* Not remembered, sorry. */
	/*__u32*/ Tcpi_last_data_recv uint32
	/*__u32*/ Tcpi_last_ack_recv uint32

	/* Metrics. */
	/*__u32*/
	Tcpi_pmtu uint32
	/*__u32*/ Tcpi_rcv_ssthresh uint32
	/*__u32*/ Tcpi_rtt uint32
	/*__u32*/ Tcpi_rttvar uint32
	/*__u32*/ Tcpi_snd_ssthresh uint32
	/*__u32*/ Tcpi_snd_cwnd uint32
	/*__u32*/ Tcpi_advmss uint32
	/*__u32*/ Tcpi_reordering uint32

	/*__u32*/
	Tcpi_rcv_rtt uint32
	/*__u32*/ Tcpi_rcv_space uint32

	/*__u32*/
	Tcpi_total_retrans uint32

	/*__u64*/
	Tcpi_pacing_rate uint64
	/*__u64*/ Tcpi_max_pacing_rate uint64
	/*__u64*/ Tcpi_bytes_acked uint64 /* RFC4898 tcpEStatsAppHCThruOctetsAcked */
	/*__u64*/ Tcpi_bytes_received uint64 /* RFC4898 tcpEStatsAppHCThruOctetsReceived */
	/*__u32*/ Tcpi_segs_out uint32 /* RFC4898 tcpEStatsPerfSegsOut */
	/*__u32*/ Tcpi_segs_in uint32 /* RFC4898 tcpEStatsPerfSegsIn */

	/*__u32*/
	Tcpi_notsent_bytes uint32
	/*__u32*/ Tcpi_min_rtt uint32
	/*__u32*/ Tcpi_data_segs_in uint32 /* RFC4898 tcpEStatsDataSegsIn */
	/*__u32*/ Tcpi_data_segs_out uint32 /* RFC4898 tcpEStatsDataSegsOut */

	/*__u64*/
	Tcpi_delivery_rate uint64

	/*__u64*/
	Tcpi_busy_time uint64 /* Time (usec) busy sending data */
	/*__u64*/ Tcpi_rwnd_limited uint64 /* Time (usec) limited by receive window */
	/*__u64*/ Tcpi_sndbuf_limited uint64 /* Time (usec) limited by send buffer */

	/*__u32*/
	Tcpi_delivered uint32
	/*__u32*/ Tcpi_delivered_ce uint32

	/*__u64*/
	Tcpi_bytes_sent uint64 /* RFC4898 tcpEStatsPerfHCDataOctetsOut */
	/*__u64*/ Tcpi_bytes_retrans uint64 /* RFC4898 tcpEStatsPerfOctetsRetrans */
	/*__u32*/ Tcpi_dsack_dups uint32 /* RFC4898 tcpEStatsStackDSACKDups */
	/*__u32*/ Tcpi_reord_seen uint32 /* reordering events seen */

	/*__u32*/
	Tcpi_rcv_ooopack uint32 /* Out-of-order packets received */

	/*__u32*/
	Tcpi_snd_wnd uint32 /* peer's advertised receive window after
	 * scaling (bytes)
	 */
}

func (t *TCPInfoLinux) common() *TCPInfo {
	var tinfo TCPInfo
	tinfo.RttMs = t.Tcpi_rtt / 1000
	tinfo.RttVarMs = t.Tcpi_rttvar / 1000
	tinfo.ReTransmitPackets = t.Tcpi_total_retrans
	//tinfo.TotalPackets = 0 // todo use connection wrapper get write bytes, then minus the not sent bytes, than divide mss
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

func IsEADDRINUSE(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}
