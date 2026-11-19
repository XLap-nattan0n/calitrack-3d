import fs from 'fs';
import { Document, NodeIO } from '@gltf-transform/core';
import { KHRDracoMeshCompression } from '@gltf-transform/extensions';
import { weld, draco, prune } from '@gltf-transform/functions';
import draco3d from 'draco3dgltf';
const map = JSON.parse(fs.readFileSync('mapping.json'));
const rows = fs.readFileSync('isa_element_parts.txt','utf8').split('\n').slice(1).filter(Boolean).map(l=>l.split('\t'));
const have = new Set(fs.readdirSync('isa_BP3D_4.0_obj_99').map(f=>f.replace('.obj','')));
const doc = new Document(); const buf = doc.createBuffer(); const scene = doc.createScene('BodyParts3D_muscles');
const rep = {};
const SC = 0.001;
for (const [code, pats] of Object.entries(map)) {
  const res = pats.map(p=>new RegExp(p,'i'));
  const names = new Set(), fjs = new Set();
  for (const [id,name,fj] of rows) if (res.some(r=>r.test(name)) && have.has(fj)) { names.add(name); fjs.add(fj); }
  rep[code] = { fma_concepts:[...names].sort(), fj_files:[...fjs].sort() };
  if (!fjs.size) continue;
  const pos=[], idx=[]; let off=0;
  for (const fj of fjs) {
    const lines = fs.readFileSync(`isa_BP3D_4.0_obj_99/${fj}.obj`,'utf8').split('\n'); let n=0;
    for (const l of lines) {
      if (l.startsWith('v ')) { const [,x,y,z]=l.trim().split(/\s+/); pos.push(+x*SC,+z*SC,-y*SC); n++; }
      else if (l.startsWith('f ')) { const t=l.trim().split(/\s+/).slice(1).map(s=>parseInt(s)-1+off); for(let i=1;i<t.length-1;i++) idx.push(t[0],t[i],t[i+1]); }
    }
    off+=n;
  }
  const P = doc.createAccessor().setType('VEC3').setArray(new Float32Array(pos)).setBuffer(buf);
  const I = doc.createAccessor().setType('SCALAR').setArray(new Uint32Array(idx)).setBuffer(buf);
  const mat = doc.createMaterial('mat_'+code).setBaseColorFactor([0.75,0.25,0.22,1]).setRoughnessFactor(0.7).setMetallicFactor(0);
  const prim = doc.createPrimitive().setAttribute('POSITION',P).setIndices(I).setMaterial(mat);
  const mesh = doc.createMesh(code).addPrimitive(prim);
  scene.addChild(doc.createNode(code).setMesh(mesh));
  rep[code].vertices_raw = pos.length/3; rep[code].triangles_raw = idx.length/3;
}
await doc.transform(weld({tolerance:1e-5}), prune());
doc.createExtension(KHRDracoMeshCompression).setRequired(true);
const io = new NodeIO().registerExtensions([KHRDracoMeshCompression]).registerDependencies({'draco3d.encoder': await draco3d.createEncoderModule(),'draco3d.decoder': await draco3d.createDecoderModule()});
await doc.transform(draco({method:'edgebreaker', quantizePosition:14, quantizeNormal:10}));
await io.write('/tmp/bp3d_out/anatomy-muscles.glb', doc);
fs.writeFileSync('report.json', JSON.stringify(rep,null,1));
console.log(fs.statSync('/tmp/bp3d_out/anatomy-muscles.glb').size);
