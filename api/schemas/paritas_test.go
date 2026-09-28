package schemas

import (
	"encoding/json"
	"testing"
)

// Tiruan struct yang DIPAKAI backend ERP untuk men-decode jawaban kita.
// Disalin apa adanya dari
// backend-super-app/internal/pkg/radiusclient/disconnect_api.go supaya tes ini
// mengunci KONTRAKNYA, bukan implementasi kita sendiri.
type tiruanUserSessionStatus struct {
	Username string `json:"username"`
	IsOnline bool   `json:"is_online"`
}

type tiruanDisconnectAPIResult struct {
	Success    bool   `json:"success"`
	Status     string `json:"status"`
	Username   string `json:"username"`
	NasIP      string `json:"nas_ip"`
	Port       int    `json:"port"`
	Output     string `json:"output"`
	ReturnCode int    `json:"return_code"`
}

// Pemanggil membaca `is_online`. Kalau kita hanya mengirim `online`,
// json.Unmarshal MENGABAIKANNYA tanpa galat dan IsOnline selalu false — sinyal
// bahaya isolir mati senyap. Ini yang mengunci hal itu.
func TestV1UserStatus_DibacaBackendSebagaiIsOnline(t *testing.T) {
	for _, hidup := range []bool{true, false} {
		b, err := json.Marshal(V1UserStatus{Username: "pelanggan01", Online: hidup, IsOnline: hidup})
		if err != nil {
			t.Fatal(err)
		}
		var lihat tiruanUserSessionStatus
		if err := json.Unmarshal(b, &lihat); err != nil {
			t.Fatal(err)
		}
		if lihat.IsOnline != hidup {
			t.Fatalf("backend membaca is_online=%v, seharusnya %v — JSON: %s", lihat.IsOnline, hidup, b)
		}
		if lihat.Username != "pelanggan01" {
			t.Fatalf("username tak terbaca: %s", b)
		}
	}
}

// `Diterima()` di backend = Success || status=="acknowledged". Kedua jalan harus
// menghasilkan kesimpulan yang sama, supaya tendangan yang BERHASIL tak pernah
// tercatat sebagai penolakan.
func TestV1DisconnectResponse_DiterimaTerbacaDuaJalan(t *testing.T) {
	kasus := []struct {
		nama     string
		resp     V1DisconnectResponse
		diterima bool
	}{
		{"ACK", V1DisconnectResponse{Status: "acknowledged", Success: true}, true},
		{"NAK", V1DisconnectResponse{Status: "rejected", Success: false}, false},
		{"tanpa sesi", V1DisconnectResponse{Status: "rejected", Code: "no-active-session", Success: false}, false},
	}
	for _, k := range kasus {
		b, err := json.Marshal(k.resp)
		if err != nil {
			t.Fatal(err)
		}
		var lihat tiruanDisconnectAPIResult
		if err := json.Unmarshal(b, &lihat); err != nil {
			t.Fatal(err)
		}
		// Persis rumus Diterima() di backend.
		got := lihat.Success || lihat.Status == "acknowledged"
		if got != k.diterima {
			t.Fatalf("%s: Diterima()=%v, seharusnya %v — JSON: %s", k.nama, got, k.diterima, b)
		}
	}
}

// `output` WAJIB ada dan memuat "Received Disconnect" saat router menjawab:
// cmd/periksa-jalur-kick memutuskan sebuah NAS "HIDUP" HANYA dari substring itu.
// Kalau kosong, SETIAP NAS dilaporkan tak menjawab.
func TestV1DisconnectResponse_OutputTerbacaAlatDiagnosa(t *testing.T) {
	resp := V1DisconnectResponse{
		Status: "acknowledged", Success: true,
		Output: "Received Disconnect-ACK Id 42 from 10.0.0.2:3799",
		NasIP:  "10.0.0.2",
	}
	b, _ := json.Marshal(resp)
	var lihat tiruanDisconnectAPIResult
	if err := json.Unmarshal(b, &lihat); err != nil {
		t.Fatal(err)
	}
	if lihat.Output == "" {
		t.Fatalf("field `output` tidak terbaca backend — JSON: %s", b)
	}
	if lihat.NasIP != "10.0.0.2" {
		t.Fatalf("field `nas_ip` tidak terbaca backend — JSON: %s", b)
	}
}
