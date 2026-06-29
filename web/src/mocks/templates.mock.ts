/**
 * Hand-authored MSW handlers for the Templates CRUD surface (Phase 7). Registered
 * alongside the other curated handlers so these deterministic fixtures answer
 * before the generated faker mocks. All mutations operate on the in-memory
 * `templates` fixture array, so create / edit / delete visibly persist for the
 * session (reset on full page reload, matching the rest of the mock layer).
 */
import { http, HttpResponse, delay } from 'msw';
import type { Template, TemplateInput } from '../api/model';
import { generatePreview, templates } from '../data/templates.fixtures';

// Match the artificial latency used by the other curated handlers so loading
// skeletons are exercised on first paint.
const LATENCY = 280;

let seq = 0;
const nextId = () => `tpl-${Date.now().toString(36)}-${(seq++).toString(36)}`;

export const templatesHandlers = [
  http.get('*/templates', async () => {
    await delay(LATENCY);
    return HttpResponse.json(templates);
  }),

  http.post('*/templates', async ({ request }) => {
    await delay(LATENCY);
    const body = (await request.json().catch(() => ({}))) as Partial<TemplateInput>;
    const created: Template = {
      id: nextId(),
      name: body.name?.trim() || 'Untitled template',
      description: body.description ?? '',
      appliesTo: body.appliesTo ?? {},
      sections: body.sections ?? [],
    };
    templates.push(created);
    return HttpResponse.json(created, { status: 201 });
  }),

  http.get('*/templates/:templateId', async ({ params }) => {
    await delay(LATENCY);
    const tpl = templates.find((t) => t.id === params.templateId);
    if (!tpl) {
      return HttpResponse.json({ code: 'not-found', message: 'Template not found' }, { status: 404 });
    }
    return HttpResponse.json(tpl);
  }),

  http.put('*/templates/:templateId', async ({ params, request }) => {
    await delay(LATENCY);
    const idx = templates.findIndex((t) => t.id === params.templateId);
    if (idx < 0) {
      return HttpResponse.json({ code: 'not-found', message: 'Template not found' }, { status: 404 });
    }
    const body = (await request.json().catch(() => ({}))) as Partial<TemplateInput>;
    const updated: Template = {
      id: templates[idx].id,
      name: body.name?.trim() || templates[idx].name,
      description: body.description ?? '',
      appliesTo: body.appliesTo ?? {},
      sections: body.sections ?? [],
    };
    templates[idx] = updated;
    return HttpResponse.json(updated);
  }),

  http.delete('*/templates/:templateId', async ({ params }) => {
    await delay(LATENCY);
    const idx = templates.findIndex((t) => t.id === params.templateId);
    if (idx >= 0) templates.splice(idx, 1);
    return new HttpResponse(null, { status: 204 });
  }),

  // Test-generate a doc from the (stored) template + an optional connector's
  // last synced snapshot. Returns a DocVersion the editor renders / diffs.
  http.post('*/templates/:templateId/preview', async ({ params, request }) => {
    await delay(LATENCY);
    const tpl = templates.find((t) => t.id === params.templateId);
    if (!tpl) {
      return HttpResponse.json({ code: 'not-found', message: 'Template not found' }, { status: 404 });
    }
    const body = (await request.json().catch(() => ({}))) as { connectorId?: string };
    return HttpResponse.json(generatePreview(tpl, body.connectorId));
  }),
];
