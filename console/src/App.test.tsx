import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";

describe("App", () => {
  it("renders the MailRelay operations identity and primary navigation", () => {
    render(<App />);
    expect(screen.getAllByLabelText("MailRelay").length).toBeGreaterThan(0);
    expect(screen.getAllByRole("navigation", { name: "主导航" }).length).toBeGreaterThan(0);
    expect(screen.getByRole("heading", { name: "仪表盘" })).toBeInTheDocument();
  });
});
