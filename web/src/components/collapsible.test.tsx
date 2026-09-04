import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
} from './collapsible';

test('trigger toggles content visibility', async () => {
  const user = userEvent.setup();

  render(
    <Collapsible>
      <CollapsibleTrigger>Toggle</CollapsibleTrigger>
      <CollapsibleContent>Hidden body</CollapsibleContent>
    </Collapsible>,
  );

  // Closed by default: content is not rendered.
  expect(screen.queryByText('Hidden body')).not.toBeInTheDocument();

  // Opening reveals the content.
  await user.click(screen.getByRole('button', { name: 'Toggle' }));
  expect(screen.getByText('Hidden body')).toBeInTheDocument();

  // Closing hides it again.
  await user.click(screen.getByRole('button', { name: 'Toggle' }));
  expect(screen.queryByText('Hidden body')).not.toBeInTheDocument();
});

test('respects defaultOpen', () => {
  render(
    <Collapsible defaultOpen>
      <CollapsibleTrigger>Toggle</CollapsibleTrigger>
      <CollapsibleContent>Visible body</CollapsibleContent>
    </Collapsible>,
  );

  expect(screen.getByText('Visible body')).toBeInTheDocument();
});
