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
| `radius-vpn-watchdog` | `radius-vpn-watchdog.timer` | tiap menit, diam kalau tunnel sehat |
| `radius-vpn-watchdog.{service,timer}` | systemd | — |
| `radius-docker-net` | `install-appliance.sh` + `radius-erp-enroll` | memindahkan Docker keluar dari 172.16.0.0/12 |

## Yang dijalankan mitra: SATU baris

```
sudo radius-erp-enroll <host-conc> <user-ppp> <sandi-ppp> <pool-nas> <api-user> <api-key> <port-api>
```

`sudo` BAGIAN DARI BARISNYA, bukan tambahan dari dokumen ini: `radius-erp-enroll`
ada di `/usr/local/sbin`, yang tidak ada di PATH pengguna biasa Debian, dan sejak
15 Okt mitra boleh masuk sebagai pengguna ber-sudo. Backend yang menyusunnya sudah
mengawali dengan `sudo` (`PerintahEnroll`) — jangan dibuang, jangan ditambah lagi.

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

## Mencobanya di lab (berperan sebagai mitra)

Appliance sungguhan: keempat skrip ada di `/usr/local/sbin` (`firstboot.sh`,
`radius-erp-enroll`, `radius-vpn-connect`, `radius-vpn-watchdog` — dipasang
`install-appliance.sh` saat golden image dibuat, bersama ketiga unit systemd), stack-nya di
`/opt/freeradius-api`, dan `.env`-nya dibuat `firstboot.sh` pada boot pertama.
Mitra tinggal menempelkan satu baris dari ERP.

Di klona repo tak ada salah satu pun dari itu, jadi tunjuk direktorinya dan
pasang `radius-vpn-connect` ke PATH — JANGAN jalankan `firstboot.sh` (lihat
bagian berikutnya):

```bash
cd ~/freeradius-api
./setup.sh && docker compose up -d          # .env lahir dari sini, bukan firstboot
sudo apt install -y xl2tpd curl             # radius-vpn-connect butuh keduanya
sudo STACK_DIR="$PWD" ./appliance/radius-erp-enroll \
  <host-conc> <user-ppp> <sandi-ppp> <pool-nas> erp <api-key> <port-api>
```

`radius-vpn-connect` dicari di sebelah skripnya dulu, baru PATH — jadi
memanggil `./appliance/radius-erp-enroll` sudah cukup, tanpa memasang apa pun.

Dua hal yang BEDA dari appliance sungguhan, supaya tidak menyimpulkan terlalu
jauh dari hasil lab:

1. `.env` lab dibuat `setup.sh`, yang mengisi `API_KEY` acak. Appliance
   sungguhan lahir dengan `API_KEY` KOSONG dari `firstboot.sh`, jadi sifat
   "menolak SEMUA panggilan API sebelum enroll" TIDAK berlaku di lab — di sini
   X-API-Key sudah hidup sejak `setup.sh`.
2. Sandi DB lab adalah bawaan `.env.example`, bukan sandi unik per instance.

Langkah VPN-nya menyentuh `/etc/xl2tpd`, `/etc/ppp`, dan rute mesin ini. Kalau
yang ingin diuji baru bagian kredensial + API, hentikan sesudah
"memastikan kredensial berlaku OK" dan jangan lanjutkan ke concentrator.

## JANGAN jalankan `firstboot.sh` di lab

Ia menulis `/var/lib/radius-appliance/firstboot-selesai`, dan tanda itu ikut ke
dalam OVA — sehingga appliance mitra lahir **tanpa** kredensial unik, memakai
apa pun yang tertinggal di `.env` lab. Semua mitra jadi kembar, dan itu tidak
punya gejala apa pun: semuanya menyala normal, dan baru terasa saat satu mitra
bisa membaca RADIUS mitra lain.

Sanitasi (langkah A3) harus membuang **keduanya**: tanda itu dan `.env`.

## Tempel baris enroll: xterm.js, bukan noVNC

Baris `radius-erp-enroll` panjangnya ~180 karakter, 112 di antaranya dua nilai
hex. Mengetiknya ulang bukan pilihan, dan di konsol Proxmox bawaan
**menempelnya juga bukan pilihan**: noVNC mengirim *scancode* — seolah ada yang
menekan keyboard — dan panel *clipboard*-nya hanya men-set clipboard remote,
yang pada guest headless tanpa `spice-vdagent` tidak dibaca siapa pun. Mitra
menempel, lalu **tidak terjadi apa-apa**: tanpa galat, tanpa teks.

`Console → xterm.js` bicara ke `serial0 socket` dan mengirim **byte**. Itu
menghapus satu kelas galat sekaligus: layout keyboard mitra tidak lagi
berpengaruh, capslock/numlock tak bisa desinkron, dan karakter tak hilang
karena ketik cepat. Tempel dengan `Ctrl+Shift+V`.

Syaratnya terbagi, dan pembagian itu yang menentukan dokumen mitra:

| | di mana | siapa |
|---|---|---|
| `console=tty0 console=ttyS0,115200n8` + `update-grub` | golden image | `install-appliance.sh` |
| perangkat Serial Port di VM | config hypervisor | **mitra**, 5 klik, VM mati |

Impor OVA Proxmox **tidak** membawa definisi serial port (importer-nya hanya
memetakan disk, CPU/RAM, NIC, firmware), jadi lima klik itu tak bisa
dihilangkan dari sisi kita. Dua hal yang menyelamatkan appliance kalau mitra
salah langkah: urutan `console=` menaruh `tty0` lebih dulu sehingga noVNC tetap
menampilkan pesan boot, dan **Display** VM tidak boleh di-set ke "Serial
terminal 0" — itu mematikan satu-satunya jalan mundur.

Tidak ada `systemctl enable serial-getty@ttyS0`: `systemd-getty-generator`
men-spawn-nya sendiri saat `console=ttyS0` ada DAN portnya memang ada. Enable
eksplisit justru meninggalkan unit `failed` di VM yang tak punya serial.

Gejala khas: layar hitam diam di xterm.js = perangkat serial belum ditambahkan.
Itu terlihat persis seperti appliance mati, jadi tulis di panduan mitra.

## Sesudah reboot, tunnel naik lagi sendiri — tiga lapis

Appliance duduk di lokasi mitra tanpa penjaga, jadi satu kedipan listrik tidak
boleh berarti RADIUS mati sampai ada yang datang mengetik ulang. Yang
memulihkan ada tiga, dan ketiganya menutup kasus yang berbeda:

| lapis | menutup | di mana |
|---|---|---|
| `autodial = yes` | appliance reboot | `[lac concentrator]` di `/etc/xl2tpd/xl2tpd.conf` |
| `redial = yes` + `redial timeout` | sesi putus sesudah sempat naik | sda. |
| `radius-vpn-watchdog.timer` | dial pertama gagal (router mitra belum siap), `max redials` habis, xl2tpd mati | unit systemd, tiap menit |

Sampai 14 Okt 2026 **tak satu pun dari ketiganya ada**, dan yang membuatnya
sulit terlihat: komentar di `radius-vpn-connect` menjanjikan sebaliknya —
`persist` + `maxfail 0` disebut membuat tunnel "naik lagi sendiri sesudah
appliance reboot". Keduanya tidak bisa melakukan itu. pppd bukan proses
mandiri di sini: ia **anak** xl2tpd di atas pty, dijalankan per-call dan
di-`TERM` saat call dibongkar. `persist` adalah keadaan satu proses pppd, dan
reboot membunuh prosesnya. Memulai sesi L2TP baru adalah tugas xl2tpd, dan
tanpa `autodial` xl2tpd start, membaca LAC, membuat FIFO kontrolnya, lalu
diam.

`max redials` ditulis **eksplisit** (32767), bukan `0`: man page xl2tpd tidak
menyebut nilai bawaannya, dan membaca `0` sebagai "tak terhingga" adalah
tebakan yang bisa berarti kebalikannya.

Watchdog-nya sengaja bodoh — kalau tidak ada alamat di `ppp`/`l2tp` dan tidak
ada `pppd` yang sedang bernegosiasi, ia mengirim `c concentrator`. Itu saja.
Ia **diam** selama `/etc/xl2tpd/xl2tpd.conf` belum memuat LAC hasil
`radius-vpn-connect`, jadi aman menyala di golden image yang belum di-enroll,
dan appliance yang sehat tidak menulis satu baris pun ke
`/var/log/radius-vpn.log`.

### Membuktikannya di lab

```bash
reboot                                    # lalu TANPA mengetik apa pun:
ip -4 -o addr show | grep -E ' (ppp|l2tp)'   # alamat tunnel harus muncul lagi
tail /var/log/radius-vpn.log                 # baris "ppp up: …" baru
ip route get <IP di pool NAS>                # harus lewat ppp0
```

Uji yang kedua lebih penting daripada yang pertama: **matikan concentrator**,
reboot appliance, baru hidupkan concentrator lima menit kemudian. Tunnel harus
naik sendiri — itu yang membuktikan lapis ketiga, dan itu pula bentuk kegagalan
yang paling mungkin terjadi sungguhan (listrik mitra pulih sebelum uplink-nya).

> **Appliance yang SUDAH terkirim tidak ikut diperbaiki sendiri.** Ia memegang
> `xl2tpd.conf` tanpa `autodial` dan tidak punya unit watchdog. Perbaikannya:
> `git pull` di `/opt/freeradius-api`, jalankan `./appliance/install-appliance.sh`,
> lalu jalankan ulang `radius-vpn-connect` dengan empat argumen yang sama
> (sandi PPP-nya hanya tampil sekali di ERP — kalau sudah hilang, daftarkan
> ulang dari ERP).

## Jaringan Docker: WAJIB di luar 172.16.0.0/12

Bawaan Docker (`docker0` 172.17.0.0/16, jaringan compose 172.18/16, 172.19/16, …)
berada di ruang manajemen VPN concentrator: pool NAS `/17` di 172.17–172.31
bergateway `.1`, alamat appliance di 172.16/16. Appliance lalu menganggap gateway
pool **alamatnya sendiri** (`ip route get 172.17.0.1` → `local … dev lo`):

- paket dari concentrator dibuang sebagai martian → REST API tak terjangkau dari
  ERP, padahal dst-nat/srcnat/forward di router ber-counter naik dan dari dalam
  appliance API menjawab 401;
- balasan auth RADIUS ke NAS di 172.17.x nyasar ke `docker0` → login pelanggan
  gagal.

Kejadian staging JKT-1, 21 Okt 2026. `radius-docker-net` memindahkan Docker ke
`198.18.0.0/15` (RFC 2544 — tak bentrok 172.16/12, 10/8, maupun LAN mitra):

```
/etc/docker/daemon.json
{ "bip": "198.18.0.1/24",
  "default-address-pools": [{ "base": "198.19.0.0/16", "size": 24 }] }
```

Dijalankan otomatis oleh `install-appliance.sh` (OVA baru) DAN `radius-erp-enroll`
(appliance dari OVA lama ikut beres saat di-enroll). Stack dibuat ulang dengan
`docker compose down` **tanpa `-v`** — volume database aman. `daemon.json` yang
sudah berisi hal lain TIDAK ditimpa; skripnya berhenti dan menyebut dua kunci yang
harus ditambahkan. Periksa saja: `sudo radius-docker-net --periksa`.
Uji: `bash appliance/test-radius-docker-net.sh`.

## Urutan di golden image (A1)

```bash
cd /opt/freeradius-api
./setup.sh                        # bootstrap raddb dari image FreeRADIUS
docker compose build              # image api DIBANGUN SEKARANG
docker compose pull db freeradius  # image dasar ikut ter-cache
apt install -y xl2tpd curl openssl qemu-guest-agent sudo
NOC_SSH_FROM="<cidr-noc>" AKUN_MITRA=<user> ./appliance/install-appliance.sh
```

`NOC_SSH_FROM` mempersempit dari mana sandi root berlaku lewat SSH. Dikosongkan
tetap boleh dan installer hanya memperingatkan — tapi sandi root SAMA di semua
appliance, jadi tanpa pembatas itu ia berlaku dari seluruh ruang alamat tunnel,
termasuk dari appliance mitra lain. `AKUN_MITRA=<user>` dipakai kalau mitra akan
masuk sebagai pengguna biasa: ia menjalankan `usermod -aG sudo` lalu
memverifikasinya (grup `sudo` TIDAK otomatis di Debian saat sandi root diisi).
Keduanya opsional; `sudo` di baris `apt` tidak.

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
