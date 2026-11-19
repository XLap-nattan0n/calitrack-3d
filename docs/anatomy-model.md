# Anatomy model (BodyParts3D) — NAT-72 / S1b

## แหล่งข้อมูล
- **BodyParts3D** — The Database Center for Life Science (DBCLS), https://dbarchive.biosciencedbc.jp/en/bodyparts3d/
- ไฟล์ที่ใช้: `isa_BP3D_4.0_obj_99.zip` (OBJ, 2,234 ไฟล์ `FJxxxx.obj`, 142.9 MB) + `isa_element_parts.txt` (FMA concept → FJ file) จาก `.../data/bodyparts3d/LATEST/`
- หมายเหตุรุ่น: โฟลเดอร์ `LATEST` บนเซิร์ฟเวอร์คือชุดข้อมูล `20130619` (BP3D 4.0, Last-Modified 2013-05-22) ไม่พบไดเรกทอรีรุ่น 2025/02/27 — ตัวเลขด้านล่างวัดจากไฟล์ชุดนี้

## License / Attribution ⚠️
- หน้าเว็บ BodyParts3D ระบุ **CC BY 4.0** และให้ attribute ว่า: *"BodyParts3D, © The Database Center for Life Science licensed under CC Attribution 4.0 International"*
- แต่ **header ภายในไฟล์ OBJ ทุกไฟล์ระบุ CC Attribution-Share Alike 2.1 Japan (BY-SA)** — ขัดกับหน้าเว็บ
- ข้อสรุปแบบระมัดระวัง: ให้ถือว่าไฟล์ GLB ที่แปลงแล้วเป็น derivative ภายใต้ **CC BY-SA 2.1 JP** จนกว่า DBCLS จะยืนยัน (ต้องแสดงที่มา + แจกจ่ายต่อภายใต้ license เดียวกัน) ต้องให้คนตัดสินใจ/ตรวจสอบกฎหมายก่อนปล่อย production
- ต้องแสดง attribution ในแอป (หน้า About/credits) — งานนี้ยังไม่ได้ทำ

## ขั้นตอนแปลง
`scripts/anatomy/build.mjs` (gltf-transform + draco3dgltf; ไม่มี Blender ในสภาพแวดล้อมนี้)
1. map FMA concept name → seed muscle code ด้วย regex ใน `scripts/anatomy/mapping.json` (ซ้าย+ขวารวมกัน)
2. อ่าน OBJ (หน่วย mm, Z-up) → แปลงเป็นเมตร, +Y up (x, z, -y), รวมเป็น 1 mesh/node ต่อ muscle code, ตั้งชื่อ node และ mesh = `muscles.code`
3. weld (1e-5) + Draco (edgebreaker, quantize position 14 bit)

## ผลลัพธ์ (นับจาก GLB จริงด้วย `scripts/anatomy/verify.mjs`, ดู `assets/anatomy/mesh-count.txt`)
- ไฟล์: `assets/anatomy/anatomy-muscles.glb` — **1,509,200 bytes (1.44 MB)** จึงเก็บใน git ได้ (<10MB)
- sha256 `19bb7987dfebf725c53f133046442cef493e97764229135ee98f6619de32b2ab`
- **24 meshes / 24 nodes** (จาก 27 muscle ใน seed), vertices 414,254, triangles 823,640 (หลัง weld)
- bounds (m): x ±0.319, y 0.032–1.506, z −0.025–0.219
- มี material เดียวสีแดงต่อ mesh (ยังไม่มี UV/texture; ยังไม่ได้ render preview เพราะไม่มี Blender)
- ชุดดิบมีกล้ามเนื้อ 93 concept ใน parts list; ไฟล์ FJ ที่ใช้ทั้งหมดรวม 142 ไฟล์

## ตาราง mapping (seed `muscles.code` → mesh)
mesh_name ที่ต้องใช้ใน DB = ค่า `code` (ตรงกับ convention ใน seed เดิม จึงไม่ต้องแก้ migration สำหรับ 24 มัดที่ map ได้)

| code | สถานะ | FJ files (L+R) | vertices (ก่อน weld) | triangles | FMA concepts |
|---|---|---|---|---|---|
| `pectoralis_major` | ✅ | 6 | 15971 | 20992 | 6 concepts |
| `pectoralis_minor` | ✅ | 2 | 13487 | 18936 | 2 concepts |
| `serratus_anterior` | ✅ | 2 | 56915 | 98424 | 2 concepts |
| `latissimus_dorsi` | ❌ ไม่มีใน BodyParts3D | 0 | - | - | 0 concepts |
| `trapezius` | ✅ | 6 | 32132 | 44196 | 6 concepts |
| `rhomboids` | ✅ | 4 | 13131 | 18816 | 4 concepts |
| `teres_major` | ✅ | 2 | 2096 | 2084 | 2 concepts |
| `erector_spinae` | ✅ | 14 | 82776 | 109792 | 14 concepts |
| `anterior_deltoid` | ✅ | 2 | 5196 | 4412 | 2 concepts |
| `lateral_deltoid` | ✅ | 2 | 1834 | 2600 | 2 concepts |
| `posterior_deltoid` | ✅ | 2 | 3343 | 3800 | 2 concepts |
| `rotator_cuff` | ✅ | 8 | 9483 | 8684 | 8 concepts |
| `biceps_brachii` | ✅ | 4 | 5574 | 6564 | 4 concepts |
| `brachialis` | ✅ | 2 | 2522 | 3492 | 2 concepts |
| `triceps_brachii` | ✅ | 6 | 6805 | 10052 | 6 concepts |
| `forearm_flexors` | ✅ | 18 | 44623 | 53368 | 16 concepts |
| `forearm_extensors` | ✅ | 22 | 40168 | 49444 | 20 concepts |
| `rectus_abdominis` | ❌ ไม่มีใน BodyParts3D | 0 | - | - | 0 concepts |
| `obliques` | ⚠️ บางส่วน | 2 | 127800 | 223652 | 2 concepts |
| `transversus_abdominis` | ❌ ไม่มีใน BodyParts3D | 0 | - | - | 0 concepts |
| `gluteus_maximus` | ✅ | 2 | 4106 | 4752 | 2 concepts |
| `gluteus_medius` | ✅ | 2 | 4620 | 6712 | 2 concepts |
| `quadriceps` | ✅ | 8 | 25638 | 42376 | 8 concepts |
| `hamstrings` | ✅ | 8 | 16897 | 28324 | 8 concepts |
| `adductors` | ✅ | 8 | 17571 | 24020 | 8 concepts |
| `hip_flexors` | ✅ | 4 | 12165 | 15524 | 4 concepts |
| `calves` | ✅ | 6 | 13369 | 22624 | 6 concepts |

## ช่องว่าง (gap)
- `latissimus_dorsi`, `rectus_abdominis`, `transversus_abdominis` — **ไม่มีใน BodyParts3D** (ค้นชื่อ FMA แล้วไม่พบ element file) → ไม่มี mesh; ห้ามใช้ของปลอมแทน
- `obliques` — มีเฉพาะ external oblique (internal oblique ไม่มี) จึง map ได้บางส่วน
- `erector_spinae` — ประกอบจาก iliocostalis/longissimus/spinalis thoracis (ไม่มี spinalis cervicis/lumborum, multifidus)
- `rotator_cuff` ครบ 4 มัด; `forearm_*` และ `hip_flexors` เป็นการจัดกลุ่มเชิง functional โดยผู้ทำ (ควรให้ผู้เชี่ยวชาญตรวจ)
- แนวทางปิดช่องว่าง: (a) ตั้ง `mesh_name = null` ให้ 3 มัดนี้ และให้ viewer แสดงว่าไม่มีโมเดล, (b) ใช้แหล่งอื่น (เช่น Z-Anatomy, CC BY-SA) เติม, (c) ให้ modeler สร้างเอง — รอการตัดสินใจ

## Reproduce
```
unzip isa_BP3D_4.0_obj_99.zip -d <dir>; cp isa_element_parts.txt <dir>/..   # ปรับ path ใน build.mjs
npm i @gltf-transform/core @gltf-transform/extensions @gltf-transform/functions draco3dgltf
node scripts/anatomy/build.mjs && node scripts/anatomy/verify.mjs
```

## Viewer (NAT-73)
- หน้า `/anatomy` (`frontend/src/components/anatomy/`), GLB สำเนาที่ `frontend/public/models/anatomy-muscles.glb` (ต้อง sync กับ `assets/anatomy/` เมื่อ rebuild), Draco decoder self-host ที่ `frontend/public/draco/`
- attribution แสดงท้ายหน้า viewer ตามข้อความ CC BY 4.0 ของ BodyParts3D — **ยังมีประเด็น BY-SA 2.1 JP ที่ต้องให้คนตัดสิน (ดูหัวข้อ License)**
- มัดที่ไม่มี mesh ตรวจจากชื่อ mesh ใน GLB จริง (ไม่ hard-code) และแสดงในกลุ่ม "No 3D model yet"
