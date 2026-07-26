import { useEffect, useRef } from "react";

export function useDocumentTitle(title: string): void {
  const titleRef = useRef(document.title);

  useEffect(() => {
    document.title = title;

    return () => {
      document.title = titleRef.current;
    };
  }, [title]);
}
