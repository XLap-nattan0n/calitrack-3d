import { NodeIO } from '@gltf-transform/core';
import { KHRDracoMeshCompression } from '@gltf-transform/extensions';
import draco3d from 'draco3dgltf';
import fs from 'fs';
const io = new NodeIO().registerExtensions([KHRDracoMeshCompression]).registerDependencies({'draco3d.decoder': await draco3d.createDecoderModule()});
const d = await io.read('/tmp/bp3d_out/anatomy-muscles.glb');
const rep = JSON.parse(fs.readFileSync('report.json'));
let tv=0,tt=0; const mn=[1e9,1e9,1e9],mx=[-1e9,-1e9,-1e9];
console.log('meshes',d.getRoot().listMeshes().length,'nodes',d.getRoot().listNodes().length);
for (const m of d.getRoot().listMeshes()) for (const p of m.listPrimitives()) {
  const v=p.getAttribute('POSITION'); tv+=v.getCount(); tt+=p.getIndices().getCount()/3;
  const a=v.getArray(); for(let i=0;i<a.length;i++){const k=i%3; mn[k]=Math.min(mn[k],a[i]); mx[k]=Math.max(mx[k],a[i]);}
  console.log(m.getName(), v.getCount(), p.getIndices().getCount()/3, rep[m.getName()].fj_files.length);
}
console.log('total verts',tv,'tris',tt,'min',mn.map(x=>x.toFixed(3)),'max',mx.map(x=>x.toFixed(3)));
console.log('missing:',Object.entries(rep).filter(([k,v])=>!v.fj_files.length).map(x=>x[0]));
