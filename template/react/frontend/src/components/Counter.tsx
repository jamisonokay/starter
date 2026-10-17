import { useEffect, useState } from "react";
import { getHello } from "../api";

export interface CounterProps {
  title?: string;
}

export function Counter({ title = "React island" }: CounterProps) {
  const [count, setCount] = useState(0);
  const [message, setMessage] = useState("loading…");

  useEffect(() => {
    getHello()
      .then((hello) => setMessage(`${hello.message} (${hello.time})`))
      .catch(() => setMessage("API is unreachable"));
  }, []);

  return (
    <div>
      <p className="out">{title}</p>
      <p className="out">{message}</p>
      <div className="row">
        <button className="btn" type="button" onClick={() => setCount((c) => c + 1)}>
          Clicked {count} times
        </button>
      </div>
    </div>
  );
}
