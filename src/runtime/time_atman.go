package runtime

//go:nosplit
func _nanotime() int64 {
	var t timeInfo
	t.load(_atman_shared_info)

	return t.nanotime()
}

//go:nosplit
func _time_now() (int64, int32) {
	var t timeInfo
	t.load(_atman_shared_info)

	return t.timeNow()
}

// timeInfo shadows time-related values stored in xenSharedInfo
// and vcpuTimeInfo structures.
type timeInfo struct {
	BootSec  int64
	BootNsec int64

	System int64 // ns since system boot / resume
	TSC    int64 // tsc value of update to System

	TSCMul   int64 // scaling factors to convert TSC to nanoseconds
	TSCShift uint8
}

// load atomically populates t from info.
func (t *timeInfo) load(info *xenSharedInfo) {
	for {
		var (
			version   = atomicload(&info.VCPUInfo[0].Time.Version)
			wcversion = atomicload(&info.WcVersion)
		)

		// The shared data is being updated, try again
		if version&1 == 1 || wcversion&1 == 1 {
			continue
		}

		t.BootSec = int64(info.WcSec)
		t.BootNsec = int64(info.WcNsec)
		t.System = int64(info.VCPUInfo[0].Time.SystemTime)
		t.TSC = int64(info.VCPUInfo[0].Time.TscTimestamp)
		t.TSCMul = int64(info.VCPUInfo[0].Time.TscToSystemMul)
		t.TSCShift = uint8(info.VCPUInfo[0].Time.TscShift)

		var (
			newversion   = atomicload(&info.VCPUInfo[0].Time.Version)
			newwcversion = atomicload(&info.WcVersion)
		)

		if newversion == version && newwcversion == wcversion {
			return
		}
	}
}

func (t *timeInfo) nsSinceSystem() int64 {
	var diffTSC = cputicks() - t.TSC

	return (diffTSC << t.TSCShift) * t.TSCMul
}

func (t *timeInfo) nanotime() int64 {
	return t.BootSec*1e9 + t.BootNsec + t.System + t.nsSinceSystem()
}

func (t *timeInfo) timeNow() (int64, int32) {
	var (
		sec  = t.BootSec
		nsec = t.BootNsec + t.System + t.nsSinceSystem()
	)

	// move whole seconds to second counter
	sec += nsec / 1e9
	nsec %= 1e9

	return sec, int32(nsec)
}
