# Cara Fix Warning "Halaman Belum Diuji Secara Visual"

## Masalah

Anda mendapatkan warning seperti ini:
```
⚠️ 1 peringatan pelacakan
Halaman belum diuji secara visual: page-002, page-003, ..., page-023
```

Ini artinya 22 halaman di project Anda belum di-screenshot dan di-submit ke Foundry untuk visual verification.

## Solusi: Command Baru `foundry visual-verify`

Saya sudah implementasikan command baru yang otomatis menangani masalah ini!

### Langkah 1: Jalankan Command Visual Verify

```bash
cd /path/to/your/project
foundry visual-verify
```

Command ini akan:
- ✅ Cek apakah visual verification commands sudah ada di workflow
- ✅ Kalau belum, otomatis tambahkan 4 commands visual verification
- ✅ Tampilkan status visual verification saat ini
- ✅ Berikan next steps yang jelas

### Output yang Diharapkan

```
✓ Berhasil menambah 4 visual verification commands

📊 Visual Verification Status:
  Total pages: 23
  Verified: 0
  Pending: 23
  Failed: 0

⚠️  23 halaman belum diverifikasi

📋 Next steps:
  1. Jalankan Foundry MCP server:
     foundry mcp --allow ALL --project /path/to/project
  
  2. Di AI agent (Claude/OpenCode/Cursor), jalankan workflow:
     - Execute visual verification commands
     - Capture screenshots untuk semua halaman
     - Submit visual reviews
```

### Langkah 2: Jalankan AI Agent untuk Execute Commands

Setelah command visual verification ditambahkan, Anda perlu jalankan AI agent untuk execute commands tersebut:

**Option A: Pakai MCP Server (Recommended)**

```bash
# Terminal 1: Start MCP server
foundry mcp --allow ALL --project /path/to/project

# Terminal 2: Di AI agent (Claude/OpenCode/Cursor)
# Agent akan otomatis detect dan execute visual verification commands
```

**Option B: Pakai Foundry UI**

```bash
# Start Foundry server
foundry serve /path/to/project

# Buka browser ke http://localhost:8080
# Klik "Run Workflow" untuk execute semua commands
```

### Langkah 3: Agent Akan Otomatis

AI agent akan:
1. ✅ Start aplikasi Anda
2. ✅ Enumerate semua routes
3. ✅ Capture screenshots di 3 viewports (desktop, tablet, mobile)
4. ✅ Submit visual reviews ke Foundry
5. ✅ Check untuk visual issues
6. ✅ Generate fix commands kalau ada issues
7. ✅ Fix issues dan re-verify

### Langkah 4: Cek Status

Setelah agent selesai, cek status lagi:

```bash
foundry visual-verify
```

Output yang diharapkan:
```
✓ Visual verification commands sudah ada di workflow

📊 Visual Verification Status:
  Total pages: 23
  Verified: 23 ✅
  Pending: 0
  Failed: 0

✅ Semua halaman sudah diverifikasi!
```

## Troubleshooting

### Problem: Command `foundry visual-verify` tidak ditemukan

**Solusi:** Rebuild Foundry binary
```bash
cd /home/handrawan/Project/Foundry
go build -o foundry ./cmd/foundry
sudo mv foundry /usr/local/bin/
# atau copy ke PATH Anda
```

### Problem: Agent tidak execute visual commands

**Solusi:** Pastikan MCP server running dengan `--allow ALL`
```bash
foundry mcp --allow ALL --project /path/to/project
```

### Problem: Screenshots tidak di-capture

**Solusi:** Pastikan aplikasi Anda bisa dijalankan
```bash
# Test manual dulu
npm start  # atau command untuk start aplikasi Anda
# Pastikan bisa diakses di browser
```

### Problem: Visual review submission gagal

**Solusi:** Cek apakah Foundry server running
```bash
foundry serve /path/to/project
# Atau pastikan MCP server running
```

## Visual Verification Commands

Command yang ditambahkan ke workflow:

1. **CMD-START-APP**
   - Start aplikasi dan verify readiness
   - Dependencies: Semua implementation commands

2. **CMD-ENUMERATE-ROUTES**
   - Discover dan validate semua routes
   - Dependencies: CMD-START-APP

3. **CMD-CAPTURE-SCREENSHOTS**
   - Capture screenshots di 3 viewports
   - Dependencies: CMD-ENUMERATE-ROUTES

4. **CMD-SUBMIT-VISUAL-REVIEWS**
   - Submit visual reviews ke Foundry
   - Dependencies: CMD-CAPTURE-SCREENSHOTS

5. **CMD-CREATE-VISUAL-FIXES** (conditional)
   - Generate fix commands dari findings
   - Dependencies: CMD-SUBMIT-VISUAL-REVIEWS

6. **CMD-RE-VISUAL-VERIFY** (conditional)
   - Re-verify setelah fixes
   - Dependencies: Fix implementation commands

## Contoh Lengkap

```bash
# 1. Fix visual verification commands
cd /path/to/your/project
foundry visual-verify

# 2. Start MCP server
foundry mcp --allow ALL

# 3. Di AI agent, bilang:
# "Tolong jalankan visual verification workflow"

# 4. Agent akan otomatis:
#    - Execute semua visual commands
#    - Capture screenshots
#    - Submit reviews
#    - Fix issues
#    - Re-verify

# 5. Cek hasil
foundry visual-verify
# Harus show "✅ Semua halaman sudah diverifikasi!"

# 6. Finalize project
foundry finalize
```

## FAQ

**Q: Apakah command ini aman untuk project yang sudah berjalan?**
A: Ya, command ini hanya menambahkan visual verification commands ke workflow. Tidak mengubah code atau workflow yang sudah ada.

**Q: Berapa lama proses visual verification?**
A: Tergantung jumlah halaman dan kecepatan agent. Biasanya 5-15 menit untuk 20-30 halaman.

**Q: Apa yang terjadi kalau ada visual issues?**
A: System akan otomatis generate fix commands, agent akan fix, lalu re-verify. Loop ini berlanjut sampai semua issues fixed atau max iterations tercapai.

**Q: Apakah saya perlu manual capture screenshots?**
A: Tidak, agent akan otomatis capture screenshots menggunakan browser automation (Playwright/Puppeteer).

**Q: Bisa skip visual verification?**
A: Bisa, tapi tidak recommended. Visual verification memastikan UI Anda sesuai dengan design rules.

## Summary

Sekarang Anda punya command baru `foundry visual-verify` yang:
- ✅ Otomatis detect dan fix warning "halaman belum diuji"
- ✅ Tambahkan visual verification commands ke workflow
- ✅ Tampilkan status visual verification yang jelas
- ✅ Berikan next steps yang actionable

Tinggal jalankan `foundry visual-verify` dan ikuti next steps-nya! 🚀
