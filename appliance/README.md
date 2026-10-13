# Appliance mitra (bukan instance NOC)

Berkas di direktori ini **hanya** untuk appliance yang dikirim ke Proxmox lokal
mitra. Instance NOC yang dikelola `radius-manager-api` **tidak memakainya**: ia
mengambil dari repo ini tepat tiga hal — biner `api/freeradius-api`,
`autoclearzombie.sh`, dan `autobackups3.sh` — lalu menulis `.env` instance
sendiri (`renderAPIEnvFile`). Tidak ada satu pun rujukan ke `setup.sh`,
`docker-compose.yml`, `initdb/`, atau direktori ini di seluruh repo manager.

Pemisahan itu disengaja: perbaikan di sini tidak bisa mengganggu instance NOC
yang sedang berjalan.

## Isinya

| berkas | dijalankan oleh | kapan |
|---|---|---|
| `install-appliance.sh` | **NOC** | sekali, di golden image (langkah A1) |
| `firstboot.sh` | systemd | otomatis, boot pertama di tempat mitra |
| `radius-appliance-firstboot.service` | systemd | — |
| `radius-erp-enroll` | **mitra** | sekali, satu baris dari layar ERP |
| `radius-vpn-connect` | `radius-erp-enroll` | dipanggil, bukan diketik |

## Yang dijalankan mitra: SATU baris

```
radius-erp-enroll <host-conc> <user-ppp> <sandi-ppp> <pool-nas> <api-user> <api-key>
```

Barisnya **disalin utuh** dari panel "Sambungkan appliance" di ERP — tidak
dirangkai tangan, dan tidak ada nilai apa pun yang perlu dibaca dari konsol
Proxmox lalu diketik ke form ERP. Itu satu-satunya perpindahan manual dalam
seluruh pemasangan, satu arah, dari ERP ke appliance.

Kenapa arahnya begitu: kalau appliance membangkitkan kredensial REST-nya
sendiri, operator harus membaca dua nilai dari layar konsol lalu mengetiknya
ulang — perpindahan kedua, arah berlawanan, rawan salah ketik, dan tak
terverifikasi sampai sinkron pertama gagal dengan 401. ERP sudah membangkitkan
sandi PPP; dua nilai ini menumpang di baris yang sama.

Sebelum enroll, appliance menolak **semua** panggilan API: `API_KEY` kosong
mematikan X-API-Key (`config.muatAPIKey`, gagal-tertutup) dan `BASIC_AUTH_*`
kosong mematikan Basic (`middleware.APIKey`). Jadi tidak ada jendela terbuka
antara boot pertama dan enroll.

## JANGAN jalankan `firstboot.sh` di lab

Ia menulis `/var/lib/radius-appliance/firstboot-selesai`, dan tanda itu ikut ke
dalam OVA — sehingga appliance mitra lahir **tanpa** kredensial unik, memakai
apa pun yang tertinggal di `.env` lab. Semua mitra jadi kembar, dan itu tidak
punya gejala apa pun: semuanya menyala normal, dan baru terasa saat satu mitra
bisa membaca RADIUS mitra lain.

Sanitasi (langkah A3) harus membuang **keduanya**: tanda itu dan `.env`.

## Urutan di golden image (A1)

```bash
cd /opt/freeradius-api
./setup.sh                        # bootstrap raddb dari image FreeRADIUS
docker compose build              # image api DIBANGUN SEKARANG
docker compose pull db freeradius  # image dasar ikut ter-cache
apt install -y xl2tpd curl openssl
./appliance/install-appliance.sh
```

`docker compose build` wajib di sini, bukan di tempat mitra: build menuntut
jaringan yang sehat (registry, dl-cdn Alpine, proxy modul Go), dan jaringan
mitra tidak bisa kamu periksa dulu — dua kali gagal di mesin sendiri, lihat
bagian berikutnya. `firstboot.sh` memeriksa image `freeradius-api-local:1` ada
**sebelum** `up`, dan melapor terang-terangan kalau tidak — bukan menggantung
tanpa penjelasan.

### Kalau `apk add` menggantung saat build

Gejalanya: `docker pull` berhasil, lalu `RUN apk add` menggantung puluhan detik
per repo dan berakhir `WARNING: fetching …: Permission denied`. Dua kali
menghentikan pemasangan (28 Sep & 13 Okt 2026), dan **penyebabnya bukan Alpine,
bukan Docker, bukan repo ini**: TTL.

Pada 13 Okt 2026 `ping` dari MikroTik lokal memperlihatkan balasan tiba dengan
**TTL 2** — entah dari aturan `change-ttl` sendiri, entah dari ISP. Sisa dua hop
itu cukup untuk router (0 hop) dan untuk host Debian (1 hop, paket untuk diri
sendiri boleh TTL 1), tapi TIDAK untuk container: host harus **meneruskan**
balasan itu ke `docker0`, penerusan mengurangi TTL satu, dan paketnya mati
sebelum sampai. Itu sebabnya `docker pull` — yang bicara dari host — lancar,
sementara `RUN apk add` di dalam build tidak pernah menerima jawaban.

Periksa dari mesinnya, tanpa masuk router:

```bash
ping -c1 -t 64 1.1.1.1    # seperti trafik host
ping -c1 -t 63 1.1.1.1    # seperti trafik container
```

Beda hasil = aturan/pemotongan TTL. Obatnya di MikroTik, bukan di Dockerfile —
pulihkan TTL paket masuk di WAN:

```
/ip firewall mangle add chain=prerouting in-interface=<WAN> \
  action=change-ttl new-ttl=set:64 place-before=0
```

Ini wajib dibereskan di setiap lokasi, bukan disiasati di build: appliance
butuh jalan keluar sungguhan juga saat BERJALAN (L2TP ke concentrator, sinkron
ke ERP), dan keduanya berangkat dari dalam container yang sama.

Prosedur lengkap beserta sanitasi dan ekspor OVA ada di
`backend-super-app/docs/SETUP-RADIUS-LOKAL-MITRA.md`.
