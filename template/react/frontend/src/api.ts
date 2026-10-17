// The Go endpoint the React app talks to. By default it is the same origin as
// the page (the Fiber server). To point at another server, build with
//   VITE_API_URL=https://api.example.com npm run build
// (that server must then send CORS headers).
const API_URL: string = import.meta.env.VITE_API_URL ?? "";

export interface Hello {
  message: string;
  time: string;
}

export async function getHello(): Promise<Hello> {
  const res = await fetch(`${API_URL}/api/hello`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return (await res.json()) as Hello;
}
