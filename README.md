# Pawtner API RESTful - MVC Architecture

Project Pawtner API yang dibangun menggunakan bahasa **Go (Golang)** dengan menerapkan pola arsitektur **MVC (Model-View-Controller)**. Project ini menggunakan **Gin** sebagai HTTP web framework/router dan **GORM** sebagai ORM untuk berinteraksi dengan database **MySQL**.

## 🚀 Teknologi yang Digunakan

* **Language:** [Go (Golang)](https://golang.org/)
* **Framework / Router:** [Gin Gonic](https://github.com/gin-gonic/gin)
* **ORM:** [GORM](https://gorm.io/)
* **Database:** [MySQL](https://www.mysql.com/)

---

## 📂 Struktur Arsitektur (MVC)

Project ini mengorganisasikan kode ke dalam beberapa direktori fungsional agar mudah dikelola, diskalakan, dan dipelihara:

```text
pawtner/
├── config/
│   └── database.go          # Konfigurasi koneksi database (GORM & MySQL) & Auto Migrate
├── controllers/
│   └── product_controller.go # Logika bisnis, validasi request, dan response JSON
├── models/
│   └── product.go           # Struktur data (Struct) dan representasi tabel database
├── routes/
│   └── routes.go            # Pengaturan endpoint API dan routing Grouping
├── .gitignore               # Konfigurasi file yang diabaikan oleh Git
├── go.mod                   # Modul dan manajemen dependencies Go
├── go.sum                   # Checksum dependencies
└── main.go                  # Titik masuk utama (Entry point) aplikasi