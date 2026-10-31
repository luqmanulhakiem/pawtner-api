# Pawtner API — Dokumentasi JSON untuk Frontend

Dokumen ini dibuat agar bisa dipakai langsung oleh AI Agent / developer frontend. Isinya: format respons, autentikasi, aturan bisnis, dan kontrak request/response setiap endpoint di `routes/routes.go`.

## 0. Aturan Umum

### Base URL & Header
- Semua endpoint diawali `/api`. Contoh: `POST /api/auth/login`.
- Body request: JSON, header `Content-Type: application/json`.
- Endpoint selain `/api/auth/*` **wajib** header `Authorization: Bearer <access_token>`.

### Format Respons (seragam)
Sukses:
```json
{ "success": true, "message": "...", "data": { } }
```
Error (tidak ada field `data`):
```json
{ "success": false, "message": "..." }
```
- Frontend cukup cek `success`. Teks `message` untuk error bisa ditampilkan ke user (bahasa Inggris, kecuali teks `insight` yang berbahasa Indonesia).
- Beberapa sukses mengembalikan `"data": null` (endpoint DELETE).
- Error validasi bind JSON (`400`) berisi pesan mentah dari validator Go, mis. `Key: 'LoginInput.Email' Error:Field validation for 'Email' failed on the 'required' tag`. Jangan di-parse; tampilkan pesan generik atau cek field di sisi client sebelum kirim.

### Autentikasi & Token
- Access token berlaku **15 menit**, refresh token **7 hari**.
- **Tidak ada endpoint refresh token** saat ini. Jika access token kedaluwarsa (`401`, `"Invalid or expired access token"`), arahkan user login ulang.
- Error autentikasi (`401`) dari middleware:
  - `Authorization header is required`
  - `Invalid authorization format. Use Bearer <token>`
  - `Invalid or expired access token`
- `401 Invalid user session` muncul jika token valid tapi klaim user tidak terbaca.

### Konvensi Data
- **Tanggal** (`date`, `start_date`, `conflict_date`, dll.): string `YYYY-MM-DD`, mis. `"2026-10-31"`.
- **Timestamp** (`created_at`, `event_at`, `resolved_at`, dst.): string ISO 8601 / RFC3339, mis. `"2026-10-31T09:00:00+07:00"`.
- **ID**: integer positif. Path param `:id` non-numerik → `400`.
- **Field `gorm.Model` (PENTING)**: model berikut mengembalikan field standar GORM dengan nama **PascalCase**, bukan snake_case:
  `ID`, `CreatedAt`, `UpdatedAt`, `DeletedAt` (selalu `null` pada data aktif).
  Berlaku untuk: **DailyMood, Period, Conflict, ConflictNote, PartnerRule, Memory, Partner (hanya `ID` dan `DeletedAt`; `created_at`/`updated_at` snake_case)**.
  Field lain di model tersebut snake_case. Contoh di bawah menampilkan bentuk persisnya.
- `PartnerView` (dipakai endpoint partner) memakai key snake_case penuh: `id`, `status`, dst. (bukan PascalCase).

### Peran Gender
- Gender: `"male"` atau `"female"` (huruf kecil; saat register input dinormalisasi).
- Fitur khusus gender:
  - **female**: mencatat periode (`/periods`, `/periods/insight`).
  - **male**: melihat periode pasangan (`/partners/active/periods*`) dan **mengirim undangan** partner.
- Fitur pasangan (`conflicts`, `rules`, `memories`, `partners/active/*`) **hanya bisa dipakai jika user punya partner aktif** (status `active`). Jika tidak → `403 "You don't have an active partner"`.
- Data conflicts/rules/memories dimiliki **relasi pasangan**, jadi kedua anggota pasangan melihat data yang sama.

### Alur Pemakaian yang Disarankan
1. `POST /auth/register` → `POST /auth/login` → simpan `access_token`.
2. Pria: `GET /partners/search?code=...` → `POST /partners/invite`. Wanita: `GET /partners/requests` → `PUT /partners/:id/accept`.
3. Setelah partner `active`, fitur conflicts/rules/memories/partner moods bisa dipakai.

---

## 1. Auth (tanpa token)

### POST `/api/auth/register`
Request:
```json
{ "full_name": "Budi", "email": "budi@mail.com", "password": "rahasia123", "gender": "male" }
```
| Field | Tipe | Wajib | Catatan |
|---|---|---|---|
| full_name | string | ya | |
| email | string | ya | harus unik |
| password | string | ya | di-hash bcrypt |
| gender | string | ya | `male` / `female` (tidak case-sensitive) |

Sukses `201` — `"Registration Successfully"`:
```json
{ "success": true, "message": "Registration Successfully",
  "data": { "id": 1, "full_name": "Budi", "email": "budi@mail.com", "unique_code": "K7M2QX9A" } }
```
`unique_code`: 8 karakter huruf besar/angka, dipakai partner untuk mencari/mengundang.

Error: `400` field kurang / `"Gender must be 'male' or 'female'"` / `"Email already registered or something wrong"`; `500` `"Failed to hash password"`.

### POST `/api/auth/login`
Request: `{ "email": "budi@mail.com", "password": "rahasia123" }` (keduanya wajib).

Sukses `200` — `"Login Successfully"`:
```json
{ "success": true, "message": "Login Successfully",
  "data": {
    "access_token": "eyJ...", "refresh_token": "eyJ...",
    "user": { "name": "Budi", "email": "budi@mail.com", "gender": "male", "unique_code": "K7M2QX9A" }
  } }
```
Catatan: login tidak mengembalikan `id` user. Di sini key nama adalah `name` (bukan `full_name`).

Error: `400` body tidak valid; `401` `"Wrong Email or Password"`; `500` gagal buat/simpan token.

---

## 2. Daily Moods — `/api/daily-moods`

Objek **DailyMood**:
```json
{ "ID": 12, "CreatedAt": "2026-10-31T07:00:00+07:00", "UpdatedAt": "2026-10-31T07:00:00+07:00", "DeletedAt": null,
  "user_id": 1, "date": "2026-10-31", "period": "morning", "mood_status": "happy", "note": "Tidur nyenyak" }
```
Enum:
- `period`: `morning` | `night`
- `mood_status`: `neutral` | `sad` | `angry` | `sick` | `happy` | `glad` (input dinormalisasi lower-case & trim)

Aturan: maksimal **1 mood per periode per hari per user**.

### POST `/api/daily-moods`
Body:
| Field | Tipe | Wajib |
|---|---|---|
| date | string `YYYY-MM-DD` | ya |
| period | `morning`/`night` | ya |
| mood_status | enum di atas | ya |
| note | string | tidak (default `""`) |

Sukses `201` `"Daily mood recorded successfully"`, `data` = DailyMood.
Error: `400` (`Invalid date format, use YYYY-MM-DD`, `Period must be 'morning' or 'night'`, `Mood status must be 'neutral', 'sad', 'angry', 'sick', 'happy', or 'glad'`); `409` `"Mood for this date and period already recorded"`.

### GET `/api/daily-moods?date=YYYY-MM-DD`
`date` opsional (filter satu hari). Urut tanggal terbaru dulu, lalu `morning` sebelum `night`.
Sukses `200` `"Daily moods fetched successfully"`, `data` = array DailyMood (bisa `[]`).
Error `400` format tanggal salah.

### GET `/api/daily-moods/:id`
Sukses `200` `"Daily mood fetched successfully"`, `data` = DailyMood. Error `404 "Daily mood not found"` (juga jika milik user lain).

### PUT `/api/daily-moods/:id`
Hanya `mood_status` dan `note` yang bisa diubah (tanggal & periode tetap).
Body: `{ "mood_status": "sad", "note": "..." }` — `mood_status` wajib.
Sukses `200` `"Daily mood updated successfully"`, `data` = DailyMood. Error `400`, `404`.

---

## 3. Periods — `/api/periods` (khusus female)

Objek **Period**:
```json
{ "ID": 3, "CreatedAt": "...", "UpdatedAt": "...", "DeletedAt": null,
  "user_id": 2, "start_date": "2026-10-01", "end_date": "2026-10-05", "note": "" }
```
`end_date` bernilai `null` jika periode **masih berlangsung**.

Error umum periode: `403 "only female users can record periods"`, `400 "end_date cannot be before start_date"`, `409 "period overlaps with an existing period record"`, `400 "Invalid date format, use YYYY-MM-DD"`, `404 "Period not found"`.

### POST `/api/periods`
Body: `{ "start_date": "2026-10-01", "end_date": "2026-10-05", "note": "" }`
| Field | Tipe | Wajib |
|---|---|---|
| start_date | string | ya |
| end_date | string / `null` / dihilangkan | tidak (kosong = masih berlangsung) |
| note | string | tidak |

Sukses `201` `"Period recorded successfully"`, `data` = Period.

### GET `/api/periods`
Riwayat milik sendiri, terbaru dulu. Sukses `200` `"Periods fetched successfully"`, `data` = array Period.

### GET `/api/periods/:id`
Sukses `200` `"Period fetched successfully"`, `data` = Period.

### PUT `/api/periods/:id`
Body sama dengan POST (semua field dikirim ulang; `end_date` dihilangkan/`null` = berlangsung lagi). Gunakan ini untuk menutup periode dengan mengisi `end_date`.
Sukses `200` `"Period updated successfully"`, `data` = Period.

### GET `/api/periods/insight?date=YYYY-MM-DD`
`date` **wajib** (umumnya tanggal hari ini di device user). Khusus female. Memberi fase siklus + deskripsi kesehatan.

Sukses `200` `"Period insight fetched successfully"`:
```json
{ "success": true, "message": "Period insight fetched successfully",
  "data": {
    "date": "2026-10-20",
    "phase": "follicular",
    "severity": "normal",
    "title": "Fase folikular (setelah haid)",
    "description": "Haid sudah selesai ...",
    "recommendations": ["Waktu yang baik untuk ...", "Perkiraan ovulasi sekitar 2026-10-14."],
    "cycle_day": 20,
    "last_period_start": "2026-10-01",
    "expected_next_period": "2026-10-29",
    "days_until_next_period": 9,
    "late_days": 0,
    "average_cycle_length": 28,
    "average_period_length": 5,
    "cycles_analyzed": 0,
    "fertile_window": { "start": "2026-10-09", "end": "2026-10-15" },
    "mood_signal": { "checked_days": 5, "negative_days": 1, "emotional": false }
  } }
```
Field:
| Field | Arti |
|---|---|
| phase | `unknown` \| `menstruation` \| `follicular` \| `ovulation` \| `luteal` \| `premenstrual` \| `late` |
| severity | `normal` \| `attention` \| `consult_doctor` — gunakan untuk warna/ikon peringatan (mis. hijau/kuning/merah) |
| title, description | teks Indonesia siap tampil |
| recommendations | array string saran, bisa kosong |
| cycle_day | hari ke-berapa dalam siklus (0 jika belum ada data) |
| last_period_start, expected_next_period | `YYYY-MM-DD` atau `null` jika belum ada data periode |
| days_until_next_period | integer, **negatif** jika sudah lewat; `null` jika belum ada data |
| late_days | hari keterlambatan (hanya >0 saat phase `late`) |
| average_cycle_length / average_period_length | hari; memakai default 28 / 5 bila data kurang |
| cycles_analyzed | jumlah siklus yang dipakai; `0` = prediksi memakai nilai default |
| fertile_window | `{start, end}` atau `null` |
| mood_signal | `checked_days` hari yang punya catatan mood (7 hari terakhir), `negative_days` hari `sad`/`angry`, `emotional` = true jika ≥2 hari negatif |

Jika belum ada data periode: `phase = "unknown"`, title `"Belum ada data haid"`, field tanggal `null`.
Error: `400 "Query 'date' is required"`, `400 Invalid date format`, `403` jika bukan female.

> Prediksi bersifat perkiraan, bukan alat kontrasepsi/diagnosis medis. Tampilkan disclaimer di UI.

---

## 4. Conflicts — `/api/conflicts` (butuh partner aktif)

Objek **Conflict**:
```json
{ "ID": 5, "CreatedAt": "...", "UpdatedAt": "...", "DeletedAt": null,
  "partner_id": 1, "created_by": 1, "title": "Salah paham", "description": "...",
  "conflict_date": "2026-10-30", "status": "ongoing",
  "resolution": "", "resolved_by": null, "resolved_at": null }
```
- `status`: `ongoing` | `resolved`. `partner_id` = ID relasi pasangan (bukan user).
- Setelah `resolved`: `resolution` terisi (boleh kosong), `resolved_by` = user id, `resolved_at` = timestamp.
- Conflict yang sudah `resolved` **tidak bisa** diubah, diselesaikan lagi, atau ditambah catatan → `409 "conflict is already resolved"`.

Objek **ConflictNote**:
```json
{ "ID": 9, "CreatedAt": "...", "UpdatedAt": "...", "DeletedAt": null,
  "conflict_id": 5, "user_id": 1, "note": "Sudah ngobrol" }
```

Error umum: `403 "You don't have an active partner"`, `404 "Conflict not found"`, `400 "Invalid conflict id"`.

### POST `/api/conflicts`
Body:
| Field | Tipe | Wajib | Catatan |
|---|---|---|---|
| title | string | ya | maks 150, tidak boleh kosong setelah trim |
| description | string | tidak | |
| conflict_date | `YYYY-MM-DD` | ya | |

Sukses `201` `"Conflict recorded successfully"`, `data` = Conflict (status `ongoing`).

### GET `/api/conflicts?status=&from=&to=`
Semua query opsional: `status` = `ongoing`|`resolved`; `from`/`to` = `YYYY-MM-DD` inklusif (berdasarkan `conflict_date`), `from` tidak boleh setelah `to`. Urut `conflict_date` terbaru dulu.
Sukses `200` `"Conflicts fetched successfully"`, `data` = array Conflict.
Error `400`: `Invalid status, use 'ongoing' or 'resolved'`, `Invalid date format, use YYYY-MM-DD`, `Query 'from' must not be after 'to'`.

### GET `/api/conflicts/:id`
Sukses `200` `"Conflict fetched successfully"`. `data` = Conflict **plus** `notes` (field Conflict ada di level yang sama, tidak bersarang):
```json
{ "ID": 5, "title": "...", "status": "ongoing", "...": "...",
  "notes": [ { "ID": 9, "conflict_id": 5, "user_id": 1, "note": "..." } ] }
```
`notes` urut dari terlama ke terbaru (timeline), `[]` jika kosong.

### PUT `/api/conflicts/:id`
Body sama dengan POST (semua dikirim ulang). Hanya untuk `ongoing`.
Sukses `200` `"Conflict updated successfully"`, `data` = Conflict. Error `400`, `404`, `409`.

### PUT `/api/conflicts/:id/resolve`
Body: `{ "resolution": "Kami sepakat ..." }` — `resolution` opsional (body tetap harus JSON valid, minimal `{}`).
Sukses `200` `"Conflict resolved successfully"`, `data` = Conflict (status `resolved`). Boleh dilakukan salah satu dari pasangan. Error `409` jika sudah resolved.

### POST `/api/conflicts/:id/notes`
Body: `{ "note": "Sudah ngobrol" }` (`note` wajib, tidak boleh kosong).
Sukses `201` `"Conflict note added successfully"`, `data` = ConflictNote. Error `409` jika conflict sudah resolved.

---

## 5. Rules — `/api/rules` (butuh partner aktif)

Aturan/SOP bersama pasangan.

Objek **PartnerRule**:
```json
{ "ID": 4, "CreatedAt": "...", "UpdatedAt": "...", "DeletedAt": null,
  "partner_id": 1, "created_by": 1, "title": "Tidak saling mendiamkan", "description": "...",
  "category": "forbidden", "action_plan": "Bicara dalam 1 jam", "status": "active" }
```
Enum:
- `category`: `allowed` (boleh dilanggar) | `forbidden` (tidak boleh dilanggar) | `tolerated` (dalam toleransi)
- `status`: `active` | `inactive`

Error umum: `403 "You don't have an active partner"`, `404 "Rule not found"`, `400 "Invalid rule id"`, `400 "Invalid category, use 'allowed', 'forbidden' or 'tolerated'"`, `400 "Invalid status, use 'active' or 'inactive'"`.

### POST `/api/rules`
| Field | Tipe | Wajib | Catatan |
|---|---|---|---|
| title | string | ya | maks 150 |
| category | enum | ya | |
| description | string | tidak | |
| action_plan | string | tidak | langkah jika aturan dilanggar |
| status | enum | tidak | default `active` |

Sukses `201` `"Rule added successfully"`, `data` = PartnerRule.

### GET `/api/rules?category=&status=`
Filter opsional `category`, `status`. Urut `ID` terbaru dulu.
Sukses `200` `"Rules fetched successfully"`, `data` = array PartnerRule.

### GET `/api/rules/:id`
Sukses `200` `"Rule fetched successfully"`.

### PUT `/api/rules/:id`
Body sama dengan POST (`title`, `category` wajib; `description`, `action_plan` dikirim ulang). **Field `status` diabaikan** di endpoint ini; gunakan `/status`.
Sukses `200` `"Rule updated successfully"`.

### PUT `/api/rules/:id/status`
Body: `{ "status": "inactive" }` (wajib, `active`/`inactive`).
Sukses `200` `"Rule status updated successfully"`, `data` = PartnerRule.

### DELETE `/api/rules/:id`
Sukses `200` `"Rule deleted successfully"`, `"data": null`.

---

## 6. Memories — `/api/memories` (butuh partner aktif)

Kenangan bersama (seperti event kalender + link dokumentasi).

Objek **Memory**:
```json
{ "ID": 7, "CreatedAt": "...", "UpdatedAt": "...", "DeletedAt": null,
  "partner_id": 1, "created_by": 1, "title": "Liburan Bali", "description": "...",
  "location": "Bali", "event_at": "2026-10-31T09:00:00+07:00", "link_url": "https://drive.google.com/..." }
```

Error umum: `403 "You don't have an active partner"`, `404 "Memory not found"`, `400 "Invalid memory id"`, `400 "Invalid link_url, use an absolute http(s) URL"`.

### POST `/api/memories`
| Field | Tipe | Wajib | Catatan |
|---|---|---|---|
| title | string | ya | maks 150 |
| event_at | string RFC3339 | ya | mis. `2026-10-31T09:00:00+07:00` (bukan `YYYY-MM-DD` saja!) |
| description | string | tidak | |
| location | string | tidak | maks 150 |
| link_url | string | tidak | harus URL absolut `http(s)`, maks 500; kosong diperbolehkan |

Sukses `201` `"Memory added successfully"`, `data` = Memory.

### GET `/api/memories?start_date=&end_date=`
Opsional, format `YYYY-MM-DD`, keduanya **inklusif** (berdasarkan `event_at`, zona waktu server). Urut `event_at` terlama dulu (cocok untuk kalender).
Sukses `200` `"Memories fetched successfully"`, `data` = array Memory.
Error `400`: `Invalid date, use format YYYY-MM-DD`, `start_date must not be after end_date`.

### GET `/api/memories/:id`
Sukses `200` `"Memory fetched successfully"`.

### PUT `/api/memories/:id`
Body sama dengan POST; semua field dikirim ulang (`title` dan `event_at` wajib). Sukses `200` `"Memory updated successfully"`.

### DELETE `/api/memories/:id`
Sukses `200` `"Memory deleted successfully"`, `"data": null`.

---

## 7. Partners — `/api/partners`

Objek **PartnerView** (sudut pandang user yang login; berisi profil *lawannya*), semua snake_case:
```json
{ "id": 1, "status": "pending", "invited_by": 1, "partner_user_id": 2,
  "full_name": "Sari", "gender": "female", "unique_code": "AB12CD34",
  "created_at": "...", "updated_at": "..." }
```
`status`: `pending` (menunggu) | `active` (berpasangan) | `rejected` (ditolak) | `disabled` (putus).
Aturan: hanya **male** yang boleh mengundang; pasangan harus lawan jenis; satu user hanya boleh satu partner aktif. Saat undangan di-accept, undangan pending lain milik keduanya otomatis `rejected`.

Objek **Partner** mentah (dikembalikan oleh `invite`, `accept`, `reject`, `disable`):
```json
{ "ID": 1, "DeletedAt": null, "status": "pending", "male_id": 1, "female_id": 2,
  "invited_by": 1, "created_at": "...", "updated_at": "..." }
```
Catatan: untuk objek ini `ID` PascalCase dan **tidak ada** profil lawan; ambil profil lewat `GET /partners/active` atau `/requests`.

### GET `/api/partners/search?code=XXXX`
Cari user berdasarkan `unique_code` (tidak case-sensitive). `code` wajib.
Sukses `200` `"User found"`:
```json
{ "data": { "full_name": "Sari", "gender": "female", "unique_code": "AB12CD34" } }
```
Error: `400 "Query 'code' is required"`, `404 "User not found"` (juga bila kode milik diri sendiri).

### POST `/api/partners/invite`
Body: `{ "unique_code": "AB12CD34" }` (wajib). Hanya male.
Sukses `201` `"Partner invitation sent successfully"`, `data` = Partner (mentah).
Error:
- `404` `"User with this unique code not found"`
- `400` `"you cannot invite yourself"`, `"gender must be 'male' or 'female' to use partner feature"`, `"partner must be of the opposite gender"`
- `403` `"only male users can send partner invitations"`
- `409` `"you already have a partner"`, `"this user already has a partner"`, `"a pending invitation already exists between you and this user"`

### GET `/api/partners/requests`
Undangan pending yang **masuk** ke user (untuk female menerima). Sukses `200` `"Partner requests fetched successfully"`, `data` = array PartnerView (`[]` jika kosong).

### GET `/api/partners/sent?status=`
Undangan yang **dikirim** user. `status` opsional: `pending`|`active`|`rejected`|`disabled`.
Sukses `200` `"Sent partner requests fetched successfully"`, `data` = array PartnerView.
Error `400` `"Status must be 'pending', 'active', 'rejected', or 'disabled'"`.

### GET `/api/partners/active`
Sukses `200` `"Active partner fetched successfully"`, `data` = PartnerView. Error `404 "You don't have an active partner"` — gunakan ini untuk menentukan apakah user sudah berpasangan.

### GET `/api/partners/active/moods?date=YYYY-MM-DD`
Mood pasangan aktif (read-only). `date` opsional.
Sukses `200` `"Partner moods fetched successfully"`, `data` = array DailyMood (bentuk sama dengan bagian 2; `user_id` = id pasangan).
Error: `403 "You don't have an active partner"`, `400` format tanggal.

### GET `/api/partners/active/periods` (khusus male)
Riwayat periode pasangan. Sukses `200` `"Partner periods fetched successfully"`, `data` = array Period.
Error: `403 "only male users can view their partner's periods"`, `403 "You don't have an active partner"`.

### GET `/api/partners/active/periods/today?date=YYYY-MM-DD` (khusus male)
`date` wajib (umumnya hari ini). Cek apakah pasangan sedang haid.
Sukses `200` `"Partner period status fetched successfully"`:
```json
{ "data": { "date": "2026-10-03", "is_on_period": true,
            "period": { "ID": 3, "user_id": 2, "start_date": "2026-10-01", "end_date": null, "note": "", "...": "..." } } }
```
Jika tidak sedang haid: `"is_on_period": false, "period": null`.
Error: `400` (`Query 'date' is required` / format), `403` seperti di atas.

### PUT `/api/partners/:id/accept`
`:id` = `id` undangan (dari `/partners/requests`). Hanya pihak yang **diundang** (bukan pengundang). Tanpa body.
Sukses `200` `"Partner invitation accepted"`, `data` = Partner (mentah, `status: "active"`).

### PUT `/api/partners/:id/reject`
Sama seperti accept. Sukses `200` `"Partner invitation rejected"`, `status: "rejected"`.

### PUT `/api/partners/:id/disable`
Memutus relasi aktif; boleh dilakukan salah satu pasangan. Sukses `200` `"Partner relationship disabled"`, `status: "disabled"`.

Error bersama accept/reject/disable:
- `400 "Invalid partner id"`
- `404 "Partner data not found"` (id tidak ada / bukan anggota relasi)
- `403 "you are not allowed to perform this action"` (pengundang mencoba accept/reject sendiri)
- `409 "invitation is no longer pending"`, `"partner relationship is not active"`, `"you already have a partner"`, `"this user already has a partner"`

---

## 8. Ringkasan Status Code

| Code | Arti |
|---|---|
| 200 | Sukses (GET/PUT/DELETE) |
| 201 | Sukses membuat data (POST) |
| 400 | Input/validasi salah |
| 401 | Token tidak ada / tidak valid / kedaluwarsa, atau login salah |
| 403 | Tidak punya hak (gender salah / belum punya partner aktif / bukan pihak yang berhak) |
| 404 | Data tidak ditemukan (termasuk data milik user/pasangan lain) |
| 409 | Konflik state (duplikat, overlap, sudah resolved, undangan tidak pending, dst.) |
| 500 | Kesalahan server |

## 9. Tabel Endpoint

| Method | Path | Gender | Partner aktif | Deskripsi |
|---|---|---|---|---|
| POST | /api/auth/register | – | – | Daftar |
| POST | /api/auth/login | – | – | Login |
| POST | /api/daily-moods | semua | – | Catat mood |
| GET | /api/daily-moods | semua | – | List mood sendiri |
| GET | /api/daily-moods/:id | semua | – | Detail mood |
| PUT | /api/daily-moods/:id | semua | – | Ubah mood |
| POST | /api/periods | female | – | Catat periode |
| GET | /api/periods | female | – | Riwayat periode |
| GET | /api/periods/insight | female | – | Insight fase siklus |
| GET | /api/periods/:id | semua* | – | Detail periode |
| PUT | /api/periods/:id | female | – | Ubah periode |
| POST | /api/conflicts | semua | ya | Buat conflict |
| GET | /api/conflicts | semua | ya | List conflict |
| GET | /api/conflicts/:id | semua | ya | Detail + notes |
| PUT | /api/conflicts/:id | semua | ya | Ubah conflict |
| PUT | /api/conflicts/:id/resolve | semua | ya | Selesaikan conflict |
| POST | /api/conflicts/:id/notes | semua | ya | Tambah catatan |
| POST | /api/rules | semua | ya | Buat aturan |
| GET | /api/rules | semua | ya | List aturan |
| GET | /api/rules/:id | semua | ya | Detail aturan |
| PUT | /api/rules/:id | semua | ya | Ubah aturan |
| PUT | /api/rules/:id/status | semua | ya | Aktif/nonaktif |
| DELETE | /api/rules/:id | semua | ya | Hapus aturan |
| POST | /api/memories | semua | ya | Buat kenangan |
| GET | /api/memories | semua | ya | List kenangan |
| GET | /api/memories/:id | semua | ya | Detail kenangan |
| PUT | /api/memories/:id | semua | ya | Ubah kenangan |
| DELETE | /api/memories/:id | semua | ya | Hapus kenangan |
| GET | /api/partners/search | semua | – | Cari by unique_code |
| POST | /api/partners/invite | male | – | Kirim undangan |
| GET | /api/partners/requests | semua | – | Undangan masuk |
| GET | /api/partners/sent | semua | – | Undangan terkirim |
| GET | /api/partners/active | semua | ya | Partner aktif |
| GET | /api/partners/active/moods | semua | ya | Mood pasangan |
| GET | /api/partners/active/periods | male | ya | Periode pasangan |
| GET | /api/partners/active/periods/today | male | ya | Status haid pasangan hari ini |
| PUT | /api/partners/:id/accept | semua | – | Terima undangan |
| PUT | /api/partners/:id/reject | semua | – | Tolak undangan |
| PUT | /api/partners/:id/disable | semua | ya | Putus relasi |

\* `GET /api/periods/:id` hanya mengembalikan periode milik sendiri; user male tidak punya periode sehingga selalu `404`.
