import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { Markdown } from './Markdown';
import type { DocAttachment } from '../../api/model';

const image: DocAttachment = {
  id: 'image',
  docId: 'doc',
  sha256: 'a'.repeat(64),
  filename: 'photo.png',
  contentType: 'image/png',
  size: 4,
  createdBy: 'user',
  createdAt: '',
  url: '/api/attachments/image/raw?exp=100&sig=test',
};
const pdf = {
  ...image,
  id: 'pdf',
  filename: 'guide.pdf',
  contentType: 'application/pdf',
  url: '/api/attachments/pdf/raw?exp=100&sig=test',
};
afterEach(cleanup);
describe('attachment Markdown', () => {
  it('resolves owned images and keeps unknown attachments out of requests', () => {
    render(
      <Markdown
        source={'![photo](attachment:image) ![missing](attachment:outside)'}
        attachments={[image]}
      />
    );
    expect(screen.getByAltText('photo')).toHaveAttribute('src', image.url);
    expect(screen.getByText('Attachment unavailable: missing')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Enlarge photo' }));
  });
  it('renders PDF Open and an opt-in iframe preview for links and image embeds', () => {
    render(<Markdown source={'[guide](attachment:pdf)'} attachments={[pdf]} />);
    expect(screen.getByRole('link', { name: 'Open PDF' })).toHaveAttribute('href', pdf.url);
    expect(screen.queryByTitle('guide.pdf')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Preview PDF' }));
    expect(screen.getByTitle('guide.pdf')).toHaveAttribute('src', pdf.url);
    fireEvent.click(screen.getByRole('button', { name: 'Hide preview' }));
    expect(screen.queryByTitle('guide.pdf')).not.toBeInTheDocument();
  });
});
