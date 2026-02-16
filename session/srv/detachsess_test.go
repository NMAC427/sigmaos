package srv

import (
	"sync/atomic"
	"testing"

	sps "sigmaos/api/spprotsrv"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/memfssrv/memfs/inode"
	"sigmaos/sigmasrv/stats"
)

type testNewSession struct {
	protsrv sps.ProtSrv
}

func (tns *testNewSession) NewSession(_ *sp.Tprincipal, _ sessp.Tsession) sps.ProtSrv {
	return tns.protsrv
}

type testProtSrv struct{}

func (ps *testProtSrv) Version(*sp.Tversion, *sp.Rversion) *sp.Rerror { return nil }
func (ps *testProtSrv) Auth(*sp.Tauth, *sp.Rauth) *sp.Rerror          { return nil }
func (ps *testProtSrv) Attach(args *sp.Tattach, _ *sp.Rattach) (sp.TclntId, *sp.Rerror) {
	return args.TclntId(), nil
}
func (ps *testProtSrv) Walk(*sp.Twalk, *sp.Rwalk) *sp.Rerror                 { return nil }
func (ps *testProtSrv) Create(*sp.Tcreate, *sp.Rcreate) *sp.Rerror           { return nil }
func (ps *testProtSrv) Open(*sp.Topen, *sp.Ropen) *sp.Rerror                 { return nil }
func (ps *testProtSrv) Watch(*sp.Twatch, *sp.Rwatch) *sp.Rerror              { return nil }
func (ps *testProtSrv) Clunk(*sp.Tclunk, *sp.Rclunk) *sp.Rerror              { return nil }
func (ps *testProtSrv) ReadF(*sp.TreadF, *sp.Rread) ([]byte, *sp.Rerror)     { return nil, nil }
func (ps *testProtSrv) WriteF(*sp.TwriteF, []byte, *sp.Rwrite) *sp.Rerror    { return nil }
func (ps *testProtSrv) Remove(*sp.Tremove, *sp.Rremove) *sp.Rerror           { return nil }
func (ps *testProtSrv) RemoveFile(*sp.Tremovefile, *sp.Rremove) *sp.Rerror   { return nil }
func (ps *testProtSrv) Stat(*sp.Trstat, *sp.Rrstat) *sp.Rerror               { return nil }
func (ps *testProtSrv) Wstat(*sp.Twstat, *sp.Rwstat) *sp.Rerror              { return nil }
func (ps *testProtSrv) Renameat(*sp.Trenameat, *sp.Rrenameat) *sp.Rerror     { return nil }
func (ps *testProtSrv) GetFile(*sp.Tgetfile, *sp.Rread) ([]byte, *sp.Rerror) { return nil, nil }
func (ps *testProtSrv) PutFile(*sp.Tputfile, []byte, *sp.Rwrite) *sp.Rerror  { return nil }
func (ps *testProtSrv) WriteRead(*sp.Twriteread, *sessp.IoVec, *sp.Rread) (*sessp.IoVec, *sp.Rerror) {
	return nil, nil
}
func (ps *testProtSrv) Detach(*sp.Tdetach, *sp.Rdetach) *sp.Rerror { return nil }

func TestRegisterDetachSessAllCoversNewSessions(t *testing.T) {
	st := newSessionTable(&testNewSession{protsrv: &testProtSrv{}})

	var called atomic.Int32
	cb := func(_ sessp.Tsession) {
		called.Add(1)
	}

	st.RegisterDetachSessAll(cb)

	sess := st.Alloc(sp.NoPrincipal(), sessp.Tsession(101), nil)
	f := sess.GetDetachSess()
	if f == nil {
		t.Fatalf("expected detach callback for newly allocated session")
	}

	f(sessp.Tsession(101))
	if called.Load() != 1 {
		t.Fatalf("expected callback to be invoked once, got %d", called.Load())
	}
}

func TestDetachCallbackRunsOnlyOnLastClientDetach(t *testing.T) {
	tns := &testNewSession{protsrv: &testProtSrv{}}
	st := newSessionTable(tns)
	ssrv := &SessSrv{
		st:    st,
		stats: stats.NewStatsDev(inode.NewInodeAlloc(sp.DEV_STATFS)),
	}

	sid := sessp.Tsession(202)
	sess := newSession(tns.protsrv, sid, nil)

	var called atomic.Int32
	var gotSid atomic.Uint64
	sess.RegisterDetachSess(func(s sessp.Tsession) {
		gotSid.Store(uint64(s))
		called.Add(1)
	})

	cid1 := sp.TclntId(1)
	cid2 := sp.TclntId(2)
	sess.AddClnt(cid1)
	sess.AddClnt(cid2)
	st.AddLastClnt(cid1, sess)
	st.AddLastClnt(cid2, sess)

	ssrv.serve(sess, sessp.NewFcallMsg(sp.NewTdetach(cid1), nil, sid, nil))
	if called.Load() != 0 {
		t.Fatalf("callback should not run while session still has clients")
	}

	ssrv.serve(sess, sessp.NewFcallMsg(sp.NewTdetach(cid2), nil, sid, nil))
	if called.Load() != 1 {
		t.Fatalf("expected callback after last client detach, got %d", called.Load())
	}
	if gotSid.Load() != uint64(sid) {
		t.Fatalf("expected callback sid %d, got %d", sid, gotSid.Load())
	}
}
