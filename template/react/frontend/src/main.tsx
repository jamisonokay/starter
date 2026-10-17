import { StrictMode, type ComponentType } from "react";
import { createRoot } from "react-dom/client";
import { Counter } from "./components/Counter";

type Props = Record<string, unknown>;

// Components that Go templates can mount with data-react="<Name>".
// Add your own here.
const registry: Record<string, ComponentType<Props>> = {
  Counter: Counter as ComponentType<Props>,
};

function mountAll(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>("[data-react]").forEach((el) => {
    if (el.dataset.mounted) return;
    const name = el.dataset.react ?? "";
    const Component = registry[name];
    if (!Component) {
      console.warn(`[react] no component registered as "${name}"`);
      return;
    }
    const props = JSON.parse(el.dataset.props ?? "{}") as Props;
    el.dataset.mounted = "true";
    createRoot(el).render(
      <StrictMode>
        <Component {...props} />
      </StrictMode>,
    );
  });
}

mountAll();

// htmx swaps new HTML into the page; mount islands inside it as well.
document.addEventListener("htmx:afterSwap", (event) => {
  if (event.target instanceof HTMLElement) mountAll(event.target);
});
