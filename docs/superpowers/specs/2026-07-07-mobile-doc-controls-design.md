# Mobile Documentation Controls Design

## Goal

Modernize the mobile documentation controls shown in the user screenshots without changing Starlight navigation behavior or the warm Nub-inspired MailRelay identity.

## Design

- Mobile table of contents becomes a quiet, borderless toolbar with a compact accent pill and a soft floating dropdown.
- Code blocks use a warm charcoal surface with high-contrast text. The copy control becomes a small circular glass button with clear hover, focus, and copied feedback states.
- Previous and next links become a flat two-column footer navigation with small uppercase metadata, restrained separators, and no card shadow.
- Desktop behavior remains unchanged except for the improved code-block palette and copy control.

## Accessibility and constraints

- Preserve native `details`, links, and copy-button interactions.
- Keep visible focus rings and minimum 40px touch targets.
- Avoid new JavaScript and dependencies; implement through the existing custom stylesheet.
- Verify at a 390px mobile viewport and with the static build checks.

