import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { DocDiff } from './DiffViewer';
import { useUi } from '../../store/ui';

const line = (text: string) =>
  screen.getByText((_, element) => element?.tagName === 'TD' && element.textContent === text);

describe('DocDiff', () => {
  beforeEach(() => useUi.setState({ diffLayout: 'unified' }));
  afterEach(cleanup);

  it('renders unchanged, removed, and added lines with their gutter markers', () => {
    render(<DocDiff before={'stable\nold value'} after={'stable\nnew value'} />);

    expect(line('stable')).toBeInTheDocument();
    expect(line('old value')).toBeInTheDocument();
    expect(line('new value')).toBeInTheDocument();
    expect(screen.getByText('−')).toBeInTheDocument();
    expect(screen.getByText('+')).toBeInTheDocument();
    expect(screen.getByText('+1')).toBeInTheDocument();
    expect(screen.getByText('−1')).toBeInTheDocument();
  });

  it('renders an empty diff without invented additions or deletions', () => {
    render(<DocDiff before="" after="" />);

    expect(screen.getByText('+0')).toBeInTheDocument();
    expect(screen.getByText('−0')).toBeInTheDocument();
  });

  it('renders every line of a whole-file rewrite', () => {
    render(<DocDiff before={'old one\nold two'} after={'new one\nnew two'} />);

    expect(line('old one')).toBeInTheDocument();
    expect(line('old two')).toBeInTheDocument();
    expect(line('new one')).toBeInTheDocument();
    expect(line('new two')).toBeInTheDocument();
    expect(screen.getByText('+2')).toBeInTheDocument();
    expect(screen.getByText('−2')).toBeInTheDocument();
  });

  it('anchors head-revision lines for provenance links', () => {
    render(<DocDiff before={'stable\nold value'} after={'stable\nnew value'} />);

    expect(document.getElementById('diff-line-2')).toContainElement(line('new value'));
  });
});
