package realtime

import (
	"encoding/json"
	"strconv"
	"testing"
)

// A paused reader: 200 positions of one channel queue as one frame, the newest.
func TestConn_PositionsLatestWins(t *testing.T) {
	conn := NewConn(Session{}, 64)
	for i := 1; i <= 200; i++ {
		conn.Deliver("fleet:x", Frame{Type: "position", Data: json.RawMessage(strconv.Itoa(i))})
	}
	frames := conn.Drain()
	if len(frames) != 1 {
		t.Fatalf("want 1 frame, got %d", len(frames))
	}
	var got wireFrame
	if err := json.Unmarshal(frames[0], &got); err != nil {
		t.Fatal(err)
	}
	if string(got.Data) != "200" || got.Seq != 1 || got.Channel != "fleet:x" {
		t.Fatalf("want the newest position as seq 1, got %+v", got)
	}
	if _, _, closed := conn.Closed(); closed {
		t.Fatal("positions alone must never close the connection")
	}
}

// A paused reader: 64 stop frames wait, the 65th closes the connection 1013.
func TestConn_QueueOverflowCloses1013(t *testing.T) {
	conn := NewConn(Session{}, 64)
	for i := 0; i < 64; i++ {
		conn.Deliver("trip:x", Frame{Type: "stop"})
	}
	if _, _, closed := conn.Closed(); closed {
		t.Fatal("64 frames fit the queue")
	}
	conn.Deliver("trip:x", Frame{Type: "stop"})
	code, _, closed := conn.Closed()
	if !closed || code != StatusTryAgainLater {
		t.Fatalf("want closed 1013, got closed=%v code=%d", closed, code)
	}
}

// seq counts per channel on one connection, queued frames before the latest-wins ones.
func TestConn_SeqPerChannel(t *testing.T) {
	conn := NewConn(Session{}, 64)
	conn.Deliver("trip:a", Frame{Type: "position"})
	conn.Deliver("trip:a", Frame{Type: "stop"})
	conn.Deliver("trip:b", Frame{Type: "task"})
	var seqs []string
	for _, raw := range conn.Drain() {
		var f wireFrame
		_ = json.Unmarshal(raw, &f)
		seqs = append(seqs, f.Channel+"/"+f.Type+"/"+strconv.FormatInt(f.Seq, 10))
	}
	want := []string{"trip:a/stop/1", "trip:b/task/1", "trip:a/position/2"}
	if len(seqs) != 3 || seqs[0] != want[0] || seqs[1] != want[1] || seqs[2] != want[2] {
		t.Fatalf("want %v, got %v", want, seqs)
	}
}
