# Plugin I18N Contract Design Acceptance

The design cut is accepted when a reviewer can confirm that the written spec:

1. requires one shared catalog and English technical fallback;
2. keeps English and Chinese as the developer baseline while allowing a valid
   English-complete third-party Module to compile with visible incomplete
   locale evidence;
3. permits future packaged locales without activating them in the current
   product;
4. derives namespace ownership from the literal v1 Module ID;
5. defines strict schema, path, placeholder, duplicate-key, and resource-limit
   behavior;
6. makes the canonical catalog digest part of sealed Generation provenance;
7. assigns compiler work to PLG-P1, Host resolution to PLG-P6, and release
   conformance to PLG-P9;
8. leaves PLG-P1 through PLG-P9 unscheduled pending explicit human action;
9. introduces no functional plugin implementation in this cut.
