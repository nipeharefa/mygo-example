# mygo-demo 0.2.0 — Worship presenter revamp

Status: approved 2026-11-21; executed on branch `feat/worship-presenter`.

## Tujuan

Naikkan versi app ke **0.2.0** (tag rilis `v0.2.0-beta.1`) sekaligus ganti
produknya: dari mock explorer Kubernetes menjadi **presenter ibadah
sederhana** — pustaka lagu + ayat, susunan ibadah (deck slide), panel
panggung, dan **jendela output** untuk proyektor (bisa fullscreen di layar
kedua, blackout, lalu dipublikasikan lewat NDI). Nama app, identifier, dan
gerbang login demo (`admin`/`admin`) tetap.

Bukan clone EasyWorship: tanpa media/video/background file, tanpa jadwal
multi-service, tanpa persistensi, tanpa jaringan. Tampilan sengaja beda:
konsol bernuansa kertas hangat + aksen amber, output selalu gelap.

## Kriteria sukses

1. `mygo.config.ts` berisi `version: "0.2.0"`; tag `v0.2.0-beta.1` memicu CI
   yang memeriksa kecocokan versi lalu menerbitkan prerelease.
2. Login demo tetap bekerja (salah password → error generik; `admin`/`admin`
   → konsol).
3. Tambah lagu/ayat dari pustaka → slide masuk deck; navigasi, pilih, geser,
   hapus, blackout berfungsi.
4. Jendela output menampilkan slide aktif, ikut berubah saat konsol mengubah
   state, fullscreen di layar kedua bila ada, blackout jadi hitam penuh.
5. NDI mempublikasikan **jendela output** dengan nama sumber
   `mygo-demo (Live)`; berhenti otomatis saat output ditutup.
6. Gate hijau: gofmt, build 3 OS, vet, `CGO_ENABLED=0 go test ./...`;
   README/AGENTS/architecture sesuai produk baru.
7. `go run .` lolos smoke manual (login, tambah lagu, next/prev, blackout,
   buka output, fullscreen, NDI).

## Pendekatan yang dipertimbangkan

- **(A) Satu jendela dengan mode "Present"** — paling murah, tapi tidak ada
  output proyektor sungguhan; inti produk hilang. Ditolak.
- **(B) Jendela output kedua + NDI (dipilih)** — MyGo mendukung
  `mygo.NewWindow` dari handler; state dibagi lewat `app`; NDI yang sudah ada
  tinggal diarahkan ke jendela output. Biaya utama: disiplin invalidasi
  lintas jendela.
- **(C) Tiga panel ala EasyWorship (schedule + preview/live split + media)** —
  terlalu besar dan terlalu mirip. Ditolak.

Untuk visual: (1) pertahankan tema biru default hanya ganti layout → bukan
"ganti tampilan"; (2) **konsol warm stone + amber, output gelap (dipilih)**;
(3) konsol gelap ala EasyWorship → terlalu mirip. Ditolak.

## UX

**Konsol** — default 1280×800, minimum 1040×660:

- Top bar: wordmark `mygo-demo` + subjudul `Worship presenter`; pil status
  `PREVIEW`/`LIVE`/`BLACK`; tombol `Open output`/`Close output`;
  `Present`/`Windowed` (hanya saat output terbuka); `Start NDI`/`Stop NDI`
  (hanya saat output terbuka dan `ndiSupported()`); `Sign out`. Strip error
  NDI di bawah top bar bila `ndiErr != ""`.
- Kiri, lebar 296: `Tabs` `Songs` | `Bible`; `SearchField`; daftar baris
  dengan tombol `Add` (label aksesibilitas `Add <judul lagu>` / `Add <ref>`);
  empty state `No songs match.` / `No verses match.`. Tab Bible menambah
  `Select` paket ayat (`Assurance`, `Comfort`, `Praise`, `Sending`).
- Tengah, fleksibel: header `Run of show` + `%d slides` + tombol `Move up`,
  `Move down`, `Remove` (label `Move slide up`, `Move slide down`,
  `Remove slide`; disabled bila tidak relevan); `List` deck dengan baris
  terpilih di-highlight; empty state `Nothing on stage yet.` + `Add a song or
  verse from the library.`
- Kanan, lebar 344: judul `On stage` + counter `%d / %d`; kartu panggung 16:9
  bergaya stage (teks slide, eyebrow section, caption); baris `Next up`;
  transport `Prev` (label `Previous slide`), `Next` (label `Next slide`),
  `Black out`/`Clear black out`; hint `← → to move · B to black out`.
- Login: kartu 380 di tengah, wordmark, judul `Sign in to lead the service.`,
  field label `Username input` / `Password input`, error `Wrong username or
  password.`, `Sign in`, hint `Demo account: admin / admin`.

**Output** — `mygo-demo — Live`, 1280×720 bila tanpa layar kedua; bila ada
layar sekunder: posisi + ukuran = bounds layar itu dan langsung fullscreen.
Isi: background stage, eyebrow section (uppercase, muted), teks slide 40pt
tengah, caption; blackout = hitam penuh tanpa teks. Shortcut di jendela
output: `→`/`↓`/`Space` next, `←`/`↑` prev, `B` blackout, `Esc` keluar
fullscreen. Shortcut konsol hanya panah (TextInput tetap memangsa panah lebih
dulu); huruf `B` hanya di output agar tidak bentrok dengan pencarian.

**Tema** (`theme.go`): konsol mengikuti terang/gelap sistem, radius 10, aksen
amber; output selalu gelap.

| Token | Konsol terang | Konsol gelap | Stage/output |
| --- | --- | --- | --- |
| Background | `#fbf8f3` | `#191614` | `#0b0a09` |
| Surface | `#f3eee4` | `#241f1b` | `#161311` |
| SurfaceHover | `#ebe4d7` | `#2e2823` | `#1d1916` |
| SurfacePressed | `#e2d9c9` | `#38312a` | `#241f1b` |
| Border | `#ddd3c2` | `#3d352c` | `#2a2521` |
| Text | `#211c16` | `#f4efe6` | `#f6f1e8` |
| TextMuted | `#6e6455` | `#a49a8b` | `#a49a8b` |
| Accent | `#b45309` | `#f59e0b` | `#f59e0b` |

`consoleTheme(c *ui.Context) *ui.Theme` menyalin `c.Theme()` (agar
prefs/TextScale tetap), timpa palet sesuai `base.Dark`, lalu
`.WithAccent(...)`. `stageTheme() *ui.Theme` membangun dari `ui.DarkTheme()`
dengan palet stage. `const stageBackgroundHex = "#0b0a09"` untuk
`WindowOptions.BackgroundColor` (hindari kedip putih). Semua literal warna
hanya boleh ada di `theme.go`.

## Data & state

`data.go`:

```go
type songSection struct{ Label, Text string } // Text: baris dipisah "\n"
type song struct{ ID, Title, Author, Key string; Sections []songSection }
type verse struct{ Ref, Text string }
type versePack struct{ Name string; Verses []verse }

var songs = []song{...}      // 5 lagu
var packs = []versePack{...} // 4 paket, 16 ayat
func songByID(id string) (song, bool)
func verseByRef(ref string) (verse, bool)
```

Konten domain publik: 5 himne PD — `Amazing Grace` (Newton, G, V1–V3),
`Holy, Holy, Holy` (Heber, D, V1–V2), `Be Thou My Vision` (trans. Byrne, Eb,
V1–V2), `All Creatures of Our God and King` (trans. Draper, D, V1),
`A Mighty Fortress Is Our God` (trans. Hedge, C, V1–V2). Ayat KJV (PD):
Assurance `Psalm 23:1, John 3:16, Romans 8:28, Philippians 4:13`; Comfort
`Isaiah 41:10, Matthew 11:28, Psalm 34:18, Psalm 46:1`; Praise `Psalm 100:4,
Psalm 150:6, Hebrews 13:15, Psalm 103:1`; Sending `Matthew 28:19, Numbers
6:24, 2 Corinthians 13:14, Isaiah 6:8`.

`deck.go`:

```go
type slideKind int
const (slideSong slideKind = iota; slideVerse)
type slide struct{ ID, Title, Section, Text string; Kind slideKind }
func (s slide) caption() string  // lagu: "Title · Section"; ayat: "Ref"
func (s slide) eyebrow() string  // Section uppercase; ayat: "SCRIPTURE"
func (s slide) firstLine() string

func (a *app) addSong(id string)
func (a *app) addVerse(ref string)
func (a *app) selectSlide(i int)
func (a *app) nextSlide()
func (a *app) prevSlide()
func (a *app) removeSlide()
func (a *app) moveSlide(delta int)
func (a *app) toggleBlackout()
func (a *app) currentSlide() (slide, bool)
func (a *app) upcomingSlide() (slide, bool)
func (a *app) clampIndex()
func (a *app) changed() // Invalidate a.win dan a.live, nil-safe
```

Slide ID = `fmt.Sprintf("s%d", a.nextID)` supaya unik saat lagu yang sama
ditambah dua kali. Next/prev clamp (tidak wrap). `removeSlide` meng-clamp
index; deck kosong → index 0 dan preview kosong. Seed deck di `newApp()`:
`addSong("amazing-grace")` + `addVerse("Psalm 23:1")` (4 slide), lalu index 0.

`app` (di `main.go`):

```go
type app struct {
    signedIn bool
    username, password, authErr string
    tab int; songQuery, verseQuery, pack string
    deck []slide; index int; blackout bool; nextID int
    deckList ui.ListState
    win, live *mygo.Window
    autoNDI bool
    ndi *ndiStream; ndiErr string
}
```

`changed()` dipanggil setiap mutasi deck/blackout; `signOut()` menutup output
+ `stopNDI`; `signIn()` mereset `authErr`/`password`/`index`.

## File

| File | Isi |
| --- | --- |
| `main.go` | `app`, `newApp`, `signIn`/`signOut`, `changed`, NDI methods, `main()` |
| `theme.go` | palet + `consoleTheme` + `stageTheme` + `stageBackgroundHex` |
| `data.go` | tipe data, konten PD/KJV, lookup |
| `deck.go` | operasi deck murni |
| `console.go` | `view`, `login`, `shell`, `topBar`, `library`, `songLibrary`, `verseLibrary`, `service`, `stagePanel`, `stageCard` |
| `output.go` | `liveView`, `openOutput`, `closeOutput`, `togglePresent`, `secondDisplay` |
| `deck_test.go` | operasi murni + integritas data |
| `view_test.go` | tester konsol (login, pustaka, deck, shortcut) |
| `output_test.go` | tester `liveView`, guard NDI |
| `main_test.go` | dihapus (tes Kubernetes lama) |

`ndi_sender.go` hanya berubah: `ndiSourceName = "mygo-demo (Live)"` dan pesan
error nil-window → `NDI output needs the live window`.
`ndi_sender_test.go`/`ndi_windows_test.go` tetap (tes nil-window hanya cek
error).

## Rencana implementasi

Kerjakan di branch `feat/worship-presenter`; setiap task diakhiri commit.
Perintah gate pakai `GOCACHE="$TMPDIR/gocache-mygo-demo"` bila sandbox
menolak cache default.

### Task 0 — Versi + catatan rencana

**Files:** `mygo.config.ts`, `docs/superpowers/plans/2026-11-21-worship-presenter.md`.

- [ ] Ubah `version: "0.1.0"` → `"0.2.0"`; simpan dokumen rencana ini.
- [ ] Cek: `grep -n 'version' mygo.config.ts` → `0.2.0`.
- [ ] Commit `chore: bump version to 0.2.0`.

### Task 1 — Data

**Files:** `data.go`, `deck_test.go` (bagian data).
**Produces:** tipe & konten di atas, `songByID`, `verseByRef`.

- [ ] Tes gagal: `TestDataIsReachable` — minimal 4 lagu, tiap lagu ≥1
  section dengan Label/Text tidak kosong, ID unik; minimal 3 paket, ref unik
  lintas paket, tiap ayat punya Ref/Text.
- [ ] Jalankan `CGO_ENABLED=0 go test ./... -run TestDataIsReachable` →
  FAIL (belum ada).
- [ ] Implementasikan `data.go` dengan 5 lagu + 4 paket sesuai daftar.
- [ ] Tes lulus; commit `feat: add worship song and scripture data`.

### Task 2 — Deck

**Files:** `deck.go`, `deck_test.go`.
**Consumes:** `songByID`, `verseByRef`; `app`, `changed()` (dideklarasikan di
Task 3 — sementara `changed` boleh no-op sampai Task 3; jaga signature final).

- [ ] Tes gagal: `TestAddSongAppendsSections` (deck kosong +
  `addSong("amazing-grace")` → 3 slide, index 0, slide[0].Section `Verse 1`,
  ID unik), `TestAddVerseAppendsOneSlide`, `TestNextPrevClamp` (index 0
  prev→0; index akhir next→tetap), `TestRemoveSlideClampsIndex` (hapus slide
  terakhir → index valid; hapus semua → index 0), `TestMoveSlideReorders`,
  `TestToggleBlackout`.
- [ ] Jalankan tes → FAIL.
- [ ] Implementasikan `deck.go`; semua mutasi memanggil `a.changed()`.
- [ ] Tes lulus; commit `feat: add run-of-show deck model`.

### Task 3 — App, tema, login

**Files:** `main.go`, `theme.go`, `view_test.go`; hapus `main_test.go`.
**Produces:** `app` final, `newApp()`, `signIn`, `signOut`, `changed`,
`consoleTheme`, `stageTheme`, `app.view` (login saja dulu), `signIn` helper
tes.

- [ ] Tes gagal di `view_test.go`: `TestLoginGate` (HasText `Sign in`, bukan
  `Run of show`), `TestLoginRejectsWrongPassword` (klik `Username
  input`/`Password input`, type, klik `Sign in` → error generik,
  `signedIn` false), `TestLoginAcceptsDemoAccount`, `TestEnterSubmitsLogin`
  (`tt.Key(0, ui.KeyEnter)`), `TestSignOut` (password & authErr kosong, login
  kembali).
- [ ] Jalankan → FAIL.
- [ ] Implementasikan `main.go` + `theme.go` (view = login berkartu baru);
  hapus `main_test.go`; pertahankan `ndi_sender.go`/`ndi_windows.go`/
  `ndi_linux.go`/`ndi_darwin.go`/`ndi_other.go` apa adanya (NDI methods
  memakai `a.live`, sementara nil).
- [ ] `go test ./...` lulus; commit
  `feat: rebuild app around the worship console`.

### Task 4 — Konsol

**Files:** `console.go`, `view_test.go`.
**Produces:** `view` = login atau `shell`; komponen sesuai UX di atas;
shortcut panah; `deckList` wiring.

- [ ] Tes gagal: `TestLibraryShowsSongs`, `TestAddSongFromLibrary` (reset
  `a.deck`, klik `Add Amazing Grace` → 3 slide, HasText `Verse 1`),
  `TestBibleTabAddsVerse` (klik `Bible`, klik `Add Psalm 23:1` → slide
  terakhir Title `Psalm 23:1`), `TestSongSearchFilters`
  (`a.songQuery = "grace"` → hanya Amazing Grace), `TestDeckArrowShortcuts`
  (`tt.Key(0, ui.KeyRight)` → index 1; `KeyLeft` → 0),
  `TestBlackoutButton` (klik `Black out` → `a.blackout`, HasText `BLACK`;
  klik `Clear black out`).
- [ ] Jalankan → FAIL.
- [ ] Implementasikan `console.go` (layout 296 | fleksibel | 344, `ui.Split`
  tidak dipakai; `ui.Divider`; `List` + `Selected`/`Key`/`Label`; empty
  state; tombol disabled).
- [ ] Tes lulus; `go build ./...`; commit `feat: build the worship console
  view`.

### Task 5 — Output + NDI

**Files:** `output.go`, `output_test.go`, `ndi_sender.go`.
**Produces:** `liveView`, `openOutput`/`closeOutput`/`togglePresent`,
`secondDisplay`, `ndiSourceName = "mygo-demo (Live)"`, guard NDI ke `a.live`.

- [ ] Tes gagal: `TestLiveViewShowsCurrentSlide` (tester `a.liveView`
  1280×720 → HasText bait pertama), `TestLiveViewFollowsNext`
  (`a.nextSlide(); tt.Frame()` → bait kedua), `TestLiveViewBlackoutHidesText`,
  `TestLiveViewShortcuts` (`KeyRight` menaikkan index),
  `TestStartNDIWithoutOutput` (`a.live == nil` → error, `a.ndi == nil`,
  `a.ndiErr != ""`), `TestNDIHiddenWithoutOutput` (tidak ada `Start NDI` di
  konsol).
- [ ] Jalankan → FAIL.
- [ ] Implementasikan `output.go`: `openOutput` membuat `mygo.NewWindow`
  (Title, 1280×720, `MinWidth 640`, `BackgroundColor: stageBackgroundHex`,
  `Content: ui.View(a.liveView)`); bila `secondDisplay()` ada →
  X/Y/Width/Height = bounds layar itu + `FullScreen: true`; `OnClosed` →
  `stopNDI`, `a.live = nil`, `changed()`; `autoNDI` → `startNDI` saat output
  dibuka. `closeOutput` men-nil-kan `a.live` sebelum `Close()`. `signOut`
  memanggil `closeOutput` + `stopNDI`. Ubah `startNDI` memakai `a.live`,
  ganti nama sumber NDI + pesan error.
- [ ] `go test ./...`, `GOOS=windows go vet ./...`, `GOOS=linux go vet ./...`
  lulus; commit `feat: present slides in an output window over NDI`.

### Task 6 — Dokumentasi

**Files:** `README.md`, `AGENTS.md`, `docs/architecture.md`.

- [ ] README ditulis ulang: deskripsi presenter, akun demo, cara jalan,
  shortcut, output + NDI (nama sumber `mygo-demo (Live)`, NDI runtime wajib,
  macOS hanya saat jendela tampil), build/test, aturan tag/versi.
- [ ] AGENTS.md: intro produk baru; aturan warna (literal hanya di
  `theme.go`); aturan jendela output (`changed()` setelah mutasi, `live`
  di-nil-kan di `OnClosed`, NDI hanya ke `live`, buat/tutup dari callback
  main-thread); data di `data.go` (setiap lagu ≥1 section, ref ayat unik);
  tabel "Where things live" diperbarui; web scaffold tetap "Known state".
- [ ] `docs/architecture.md` ditulis ulang: state, view tree, tema, output
  window, NDI, data, daftar tes, drift yang diketahui (tanpa persistensi,
  scaffold web mati, teks PD/KJV).
- [ ] Commit `docs: describe the worship presenter`.

### Task 7 — Gate, smoke, rilis

- [ ] `test -z "$(gofmt -l *.go)"`; `CGO_ENABLED=0 go build ./...`;
  `GOOS=darwin go build ./...`; `GOOS=windows go build ./...`;
  `GOOS=linux CGO_ENABLED=0 go build ./...`; `go vet ./...`;
  `CGO_ENABLED=0 go test ./...`.
- [ ] Smoke `go run .`: login; tambah/hapus/geser; panah; blackout; buka
  output (layar kedua → fullscreen); ubah slide → output ikut; `Esc` keluar
  fullscreen; `MYGO_NDI=1` + Start NDI (OBS/NDI monitor) → stream berubah;
  tutup output → NDI berhenti; ganti tampilan sistem terang/gelap; sign out.
- [ ] Merge `feat/worship-presenter` ke `main` (fast-forward), lalu
  `git tag -a v0.2.0-beta.1 -m "mygo-demo 0.2.0-beta.1: worship presenter"`;
  `git push origin main v0.2.0-beta.1`.
- [ ] Cek workflow rilis: tag base `0.2.0` = `mygo.config.ts`; release
  prerelease berisi `.dmg`, `-setup-.exe`, `.deb`, `.tar.gz`.

## Edge case & failure mode (wajib ada tesnya)

- Deck kosong: next/prev/remove/move aman, preview & output menampilkan empty
  state, `Move`/`Remove` disabled.
- Satu slide: next dan prev tidak mengubah index.
- Hapus slide terakhir: index jatuh ke slide sebelumnya yang valid, bukan
  out-of-range.
- Lagu sama ditambah dua kali: ID slide unik (tidak ada panic List `Key`).
- Blackout tetap berlaku saat next/prev; blackout tidak menghapus teks state.
- Output ditutup saat NDI jalan: sender berhenti, `a.live == nil`, konsol
  kembali ke `PREVIEW`, tidak ada panic saat `changed()`.
- Sign out saat output terbuka: output ditutup, NDI berhenti, sesi kembali ke
  login.
- Tidak ada layar kedua: output tetap terbuka windowed, tanpa error.
- Panah di kolom pencarian: caret berpindah, slide tidak (TextInput memangsa
  panah lebih dulu).
- macOS NDI saat output minimized/off-screen: `ndiErr` tampil di strip, app
  tidak crash.
- Lebar minimum 1040: teks slide wrap, tidak terpotong; `gofmt`/vet bersih.

## Review Focus

1. Navigasi di batas deck (kosong, satu slide, hapus slide terakhir) —
   pengguna mengharapkan tombol mati/aman, bukan index liar.
2. Output ditutup pengguna saat NDI streaming — harus berhenti bersih tanpa
   panic/goroutine menggantung.
3. Shortcut panah saat fokus di SearchField — harus mengedit teks, bukan
   memindah slide.
4. Fullscreen di layar kedua yang dicabut/diubah — output tidak boleh hilang;
   minimal tetap windowed dan `changed()` aman.
5. Sign out saat output terbuka — konten tidak boleh tertinggal di layar.
6. Lagu ganda dan deck kosong setelah dihapus semua — `List` tidak boleh
   panic karena ID/indeks.

## Asumsi

- Teks himne domain publik & KJV; tidak ada aset media, jaringan, atau
  lisensi CCLI.
- Deck hanya di memori (reset saat app ditutup) — konsisten dengan sifat demo.
- Tidak ada state "preview vs live" terpisah: slide terpilih = slide live
  (versi sederhana).
- MyGo tetap `v0.10.0` (rilis terbaru; tidak ada tag lebih baru), jadi bump
  versi hanya di app.
- Nama `mygo-demo`, identifier `com.example.mygodemo`, `StateKey: "main"`
  dipertahankan.
