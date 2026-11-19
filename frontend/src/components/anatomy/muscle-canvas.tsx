"use client";
import { Suspense, useEffect, useMemo, useRef } from "react";
import * as THREE from "three";
import { Canvas, useFrame, useThree } from "@react-three/fiber";
import { OrbitControls, useGLTF } from "@react-three/drei";
import type { OrbitControls as OrbitControlsImpl } from "three-stdlib";
import {
  ANATOMY_MODEL_URL, CAMERA_DISTANCE, DRACO_DECODER_PATH, HIGHLIGHT_COLORS, MODEL_CENTER,
  type Highlight, type ViewPreset,
} from "@/constants/anatomy";

export interface MuscleSceneProps {
  /** mesh name (= muscles.code) -> highlight role */
  highlights: Record<string, Highlight>;
  view: ViewPreset;
  /** bumped to re-trigger the camera animation even when the preset is unchanged */
  viewNonce: number;
  onSelect: (meshName: string | null) => void;
  /** reports the mesh names actually present in the GLB */
  onMeshes: (names: string[]) => void;
}

function Model({ highlights, onSelect, onMeshes }: Pick<MuscleSceneProps, "highlights" | "onSelect" | "onMeshes">) {
  const { scene } = useGLTF(ANATOMY_MODEL_URL, DRACO_DECODER_PATH);
  const invalidate = useThree((s) => s.invalidate);
  const root = useMemo(() => scene.clone(true), [scene]);
  const meshes = useMemo(() => {
    const out: THREE.Mesh[] = [];
    root.traverse((o) => {
      if ((o as THREE.Mesh).isMesh) {
        const mesh = o as THREE.Mesh;
        mesh.material = new THREE.MeshStandardMaterial({ color: HIGHLIGHT_COLORS.idle, roughness: 0.6, metalness: 0 });
        out.push(mesh);
      }
    });
    return out;
  }, [root]);

  useEffect(() => {
    onMeshes(meshes.map((m) => m.name));
  }, [meshes, onMeshes]);

  const anyHighlight = Object.keys(highlights).length > 0;
  useEffect(() => {
    for (const mesh of meshes) {
      const mat = mesh.material as THREE.MeshStandardMaterial;
      const h = highlights[mesh.name];
      mat.color.set(h ? HIGHLIGHT_COLORS[h] : HIGHLIGHT_COLORS.idle);
      // fade non-highlighted muscles so highlighted ones read clearly
      mat.transparent = anyHighlight && !h;
      mat.opacity = anyHighlight && !h ? 0.35 : 1;
      mat.depthWrite = !mat.transparent;
      mat.needsUpdate = true;
    }
    invalidate();
  }, [meshes, highlights, anyHighlight, invalidate]);

  useEffect(() => () => meshes.forEach((m) => (m.material as THREE.Material).dispose()), [meshes]);

  return (
    <primitive
      object={root}
      onClick={(e: { stopPropagation: () => void; object: THREE.Object3D }) => {
        e.stopPropagation();
        onSelect(e.object.name);
      }}
      onPointerOver={(e: { stopPropagation: () => void }) => {
        e.stopPropagation();
        document.body.style.cursor = "pointer";
      }}
      onPointerOut={() => {
        document.body.style.cursor = "";
      }}
    />
  );
}

function CameraRig({ view, viewNonce }: Pick<MuscleSceneProps, "view" | "viewNonce">) {
  const controls = useRef<OrbitControlsImpl>(null);
  const { camera, invalidate } = useThree();
  const target = useRef<THREE.Vector3 | null>(null);
  const center = useMemo(() => new THREE.Vector3(...MODEL_CENTER), []);

  useEffect(() => {
    const z = view === "front" ? CAMERA_DISTANCE : -CAMERA_DISTANCE;
    target.current = new THREE.Vector3(center.x, center.y, center.z + z);
    invalidate();
  }, [view, viewNonce, center, invalidate]);

  useFrame((_, dt) => {
    const t = target.current;
    if (!t) return;
    // slerp around the centre so the camera orbits rather than cutting through the model
    const from = camera.position.clone().sub(center);
    const to = t.clone().sub(center);
    const dist = THREE.MathUtils.lerp(from.length(), to.length(), 0.2);
    const q = new THREE.Quaternion().setFromUnitVectors(from.clone().normalize(), to.clone().normalize());
    const step = new THREE.Quaternion().slerp(q, Math.min(1, dt * 6));
    from.normalize().applyQuaternion(step).multiplyScalar(dist);
    camera.position.copy(center).add(from);
    controls.current?.update();
    if (camera.position.distanceTo(t) < 0.01) {
      camera.position.copy(t);
      controls.current?.update();
      target.current = null;
    }
    invalidate();
  });

  return (
    <OrbitControls
      ref={controls}
      target={MODEL_CENTER}
      enablePan={false}
      enableDamping={false}
      minDistance={0.6}
      maxDistance={4.5}
      onStart={() => { target.current = null; }}
    />
  );
}

export default function MuscleCanvas(props: MuscleSceneProps) {
  return (
    <Canvas
      frameloop="demand"
      dpr={[1, 2]}
      camera={{ fov: 35, near: 0.05, far: 20, position: [MODEL_CENTER[0], MODEL_CENTER[1], MODEL_CENTER[2] + CAMERA_DISTANCE] }}
      onPointerMissed={() => props.onSelect(null)}
      style={{ touchAction: "none" }}
    >
      <ambientLight intensity={0.9} />
      <directionalLight position={[2, 3, 3]} intensity={1.6} />
      <directionalLight position={[-2, 2, -3]} intensity={1.1} />
      <Suspense fallback={null}>
        <Model highlights={props.highlights} onSelect={props.onSelect} onMeshes={props.onMeshes} />
      </Suspense>
      <CameraRig view={props.view} viewNonce={props.viewNonce} />
    </Canvas>
  );
}
