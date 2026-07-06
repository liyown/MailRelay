'use client';

import type { ReactNode } from 'react';
import { RootProvider } from 'fumadocs-ui/provider/next';
import StaticSearchDialog from './search';

export function Provider({ children }: { children: ReactNode }) {
  return <RootProvider search={{ SearchDialog: StaticSearchDialog }}>{children}</RootProvider>;
}
