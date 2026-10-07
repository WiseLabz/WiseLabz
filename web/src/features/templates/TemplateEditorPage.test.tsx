import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLanguagePreference } from '../../i18n';
import { TemplateEditorPage } from './TemplateEditorPage';
import type { Template } from '../../api/model';

const mockTemplate: Template = {
  id: 'tpl-1',
  name: 'Storage Template',
  description: 'Documents storage appliances',
  appliesTo: { category: 'storage', type: 'custom' },
  sections: [
    { title: 'Overview', order: 0, body: 'Storage info: {{node.storage}}' },
  ],
  currentVersion: 1,
};

const putTemplates = vi.fn().mockResolvedValue(mockTemplate);

vi.mock('../../api/generated/templates/templates', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/generated/templates/templates')>();
  return {
    ...actual,
    useGetTemplatesTemplateId: () => ({
      data: mockTemplate,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    }),
    putTemplatesTemplateId: (...args: unknown[]) => putTemplates(...args),
    postTemplatesTemplateIdPreview: vi.fn(),
  };
});

vi.mock('../../hooks/useRole', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../hooks/useRole')>();
  return {
    ...actual,
    useRole: () => ({ isOperator: true, isAdmin: true }),
    useIsInstanceAdmin: () => true,
    useConnectorRole: () => 'operator',
  };
});

describe('TemplateEditorPage categories (#513)', () => {
  let queryClient: QueryClient;

  beforeEach(async () => {
    queryClient = new QueryClient();
    await setLanguagePreference('en');
  });

  afterEach(async () => {
    await setLanguagePreference('en');
  });

  it('renders all 8 categories in English in the template category select', async () => {
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/templates/tpl-1']}>
          <Routes>
            <Route path="/templates/:id" element={<TemplateEditorPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    const select = screen.getByRole('combobox', { name: /connector category/i });
    expect(select).toBeInTheDocument();

    const options = Array.from(select.querySelectorAll('option')).map((o) => ({
      value: o.value,
      text: o.textContent?.trim(),
    }));

    expect(options).toEqual([
      { value: '', text: 'Any category' },
      { value: 'virtualization', text: 'Virtualization' },
      { value: 'containers_paas', text: 'Containers' },
      { value: 'networking', text: 'Networking' },
      { value: 'dns', text: 'DNS' },
      { value: 'storage', text: 'Storage' },
      { value: 'monitoring', text: 'Monitoring' },
      { value: 'media', text: 'Media' },
      { value: 'other', text: 'Other' },
    ]);

    expect(select).toHaveValue('storage');
  });

  it('renders all 8 categories in Portuguese (Brazil) in the template category select', async () => {
    await setLanguagePreference('pt-BR');

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/templates/tpl-1']}>
          <Routes>
            <Route path="/templates/:id" element={<TemplateEditorPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    const select = screen.getByRole('combobox', { name: /categoria do conector/i });
    expect(select).toBeInTheDocument();

    const options = Array.from(select.querySelectorAll('option')).map((o) => ({
      value: o.value,
      text: o.textContent?.trim(),
    }));

    expect(options).toEqual([
      { value: '', text: 'Qualquer categoria' },
      { value: 'virtualization', text: 'Virtualização' },
      { value: 'containers_paas', text: 'Contêineres' },
      { value: 'networking', text: 'Rede' },
      { value: 'dns', text: 'DNS' },
      { value: 'storage', text: 'Armazenamento' },
      { value: 'monitoring', text: 'Monitoramento' },
      { value: 'media', text: 'Mídia' },
      { value: 'other', text: 'Outros' },
    ]);
  });

  it('allows selecting a new category and updating draft', async () => {
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/templates/tpl-1']}>
          <Routes>
            <Route path="/templates/:id" element={<TemplateEditorPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    const select = screen.getByRole('combobox', { name: /connector category/i });
    fireEvent.change(select, { target: { value: 'monitoring' } });
    expect(select).toHaveValue('monitoring');

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(putTemplates).toHaveBeenCalled());
    expect(putTemplates).toHaveBeenCalledWith(
      'tpl-1',
      expect.objectContaining({
        appliesTo: expect.objectContaining({ category: 'monitoring' }),
      }),
    );
  });
});
