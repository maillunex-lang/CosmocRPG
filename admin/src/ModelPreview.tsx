// Предпросмотр 3D-модели (GLB) в админке. three.js подгружается отдельным куском только при открытии предпросмотра.
import { useEffect, useRef, useState } from "react";

export function ModelPreview({ url }: { url: string }) {
  const host = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const [message, setMessage] = useState("");

  useEffect(() => {
    let disposed = false;
    let cleanup = () => {};
    (async () => {
      try {
        const [THREE, { GLTFLoader }, { OrbitControls }] = await Promise.all([
          import("three"),
          import("three/examples/jsm/loaders/GLTFLoader.js"),
          import("three/examples/jsm/controls/OrbitControls.js"),
        ]);
        const res = await fetch(url, { credentials: "same-origin" });
        if (!res.ok) throw new Error(`файл недоступен (${res.status})`);
        const buf = await res.arrayBuffer();
        if (disposed || !host.current) return;
        const el = host.current;
        const w = el.clientWidth || 480;
        const h = 320;
        const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
        renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
        renderer.setSize(w, h);
        el.appendChild(renderer.domElement);
        const scene = new THREE.Scene();
        scene.add(new THREE.AmbientLight(0xffffff, 0.9));
        const sun = new THREE.DirectionalLight(0xffffff, 2.2);
        sun.position.set(2, 3, 2);
        scene.add(sun);
        const camera = new THREE.PerspectiveCamera(40, w / h, 0.01, 200);
        const gltf = await new Promise<{ scene: import("three").Object3D }>((ok, fail) => new GLTFLoader().parse(buf, "", ok, fail));
        if (disposed) { renderer.dispose(); return; }
        scene.add(gltf.scene);
        const box = new THREE.Box3().setFromObject(gltf.scene);
        const size = box.getSize(new THREE.Vector3());
        const center = box.getCenter(new THREE.Vector3());
        const r = Math.max(size.x, size.y, size.z) || 1;
        camera.position.copy(center).add(new THREE.Vector3(r * 1.1, r * 0.7, r * 1.4));
        camera.lookAt(center);
        const controls = new OrbitControls(camera, renderer.domElement);
        controls.target.copy(center);
        controls.autoRotate = true;
        controls.autoRotateSpeed = 1.5;
        controls.update();
        let raf = 0;
        const loop = () => { controls.update(); renderer.render(scene, camera); raf = requestAnimationFrame(loop); };
        loop();
        setStatus("ready");
        setMessage(`Размер модели: ${size.x.toFixed(2)} × ${size.y.toFixed(2)} × ${size.z.toFixed(2)}`);
        cleanup = () => { cancelAnimationFrame(raf); controls.dispose(); renderer.dispose(); renderer.domElement.remove(); };
      } catch (e) {
        if (!disposed) { setStatus("error"); setMessage(e instanceof Error ? e.message : "не удалось показать модель"); }
      }
    })();
    return () => { disposed = true; cleanup(); };
  }, [url]);

  return (
    <div className="model-preview">
      <div ref={host} aria-label="Предпросмотр 3D-модели" />
      {status === "loading" && <p className="muted">Загрузка модели…</p>}
      {status === "error" && <p className="error">Предпросмотр недоступен: {message}</p>}
      {status === "ready" && <p className="muted">{message}. Вращайте мышью или пальцем.</p>}
    </div>
  );
}
