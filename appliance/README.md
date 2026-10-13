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

`docker compose build` wajib di sini, bukan di tempat mitra: appliance bisa
berada di jaringan dengan egress tersaring — terbukti 28 Sep 2026 saat
`apk add` gagal ke `dl-cdn.alpinelinux.org` meski Docker Hub jalan. `firstboot.sh`
memeriksa image `freeradius-api-local:1` ada **sebelum** `up`, dan melapor
terang-terangan kalau tidak — bukan menggantung tanpa penjelasan.

Prosedur lengkap beserta sanitasi dan ekspor OVA ada di
`backend-super-app/docs/SETUP-RADIUS-LOKAL-MITRA.md`.
