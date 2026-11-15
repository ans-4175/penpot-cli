# Contoh: carousel 5 slide (1080×1350)

Contoh netral (tema: design system) yang dibangun langsung di Penpot. Isi file =
bagian slide saja; gabungkan dengan helper saat dijalankan. Ganti copy, warna
(`C` di `lib.js`), dan footer (`yourname.com`) dengan milikmu.

    python3 ../make_assets.py > /tmp/li/assets.json     # ikon lucide (butuh internet)
    # satu kali: page + aset ke storage
    { echo "const ASSETS = $(cat /tmp/li/assets.json); storage.liAssets = JSON.stringify(ASSETS);"; cat ../lib.js; echo 'const pg = penpot.createPage(); pg.name = "Carousel"; await penpot.openPage(pg); return pg.id;'; } | penpot-cli exec -
    # tiap slide (assets sudah di storage → lib.js membacanya)
    cat ../lib.js s1.js | penpot-cli exec -
    cat ../lib.js s2.js | penpot-cli exec -
    ...
    penpot-cli sync      # bukti tersimpan, setelah tiap slide

Catatan: s4-a/b/c = slide 4 dipecah 3 exec (satu kartu per exec) supaya < 30 s.
Slide 4 butuh `CARDS`/`CARD` yang ada di s4-a.js; s4-b/c dijalankan dengan
definisi yang sama (lihat file). Font: Plus Jakarta Sans + JetBrains Mono harus
tersedia di Penpot.
