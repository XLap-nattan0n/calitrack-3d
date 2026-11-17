-- Reference seed (English): equipment, muscle groups, muscles.
-- Idempotent and non-destructive (ON CONFLICT DO NOTHING).
-- mesh_name follows the convention "mesh name == muscle code" (plan §6); revisit after spike S1.

insert into public.equipment (code, name_en) values
  ('bodyweight',       'Bodyweight'),
  ('pullup_bar',       'Pull-up Bar'),
  ('parallel_bars',    'Parallel Bars'),
  ('resistance_bands', 'Resistance Bands')
on conflict (code) do nothing;

insert into public.muscle_groups (code, name_en) values
  ('chest',     'Chest'),
  ('back',      'Back'),
  ('shoulders', 'Shoulders'),
  ('arms',      'Arms'),
  ('core',      'Core'),
  ('legs',      'Legs')
on conflict (code) do nothing;

insert into public.muscles (group_id, code, name_en, mesh_name)
select g.id, m.code, m.name_en, m.code
from (values
  ('chest',     'pectoralis_major',     'Pectoralis Major'),
  ('chest',     'pectoralis_minor',     'Pectoralis Minor'),
  ('chest',     'serratus_anterior',    'Serratus Anterior'),
  ('back',      'latissimus_dorsi',     'Latissimus Dorsi'),
  ('back',      'trapezius',            'Trapezius'),
  ('back',      'rhomboids',            'Rhomboids'),
  ('back',      'teres_major',          'Teres Major'),
  ('back',      'erector_spinae',       'Erector Spinae'),
  ('shoulders', 'anterior_deltoid',     'Anterior Deltoid'),
  ('shoulders', 'lateral_deltoid',      'Lateral Deltoid'),
  ('shoulders', 'posterior_deltoid',    'Posterior Deltoid'),
  ('shoulders', 'rotator_cuff',         'Rotator Cuff'),
  ('arms',      'biceps_brachii',       'Biceps Brachii'),
  ('arms',      'brachialis',           'Brachialis'),
  ('arms',      'triceps_brachii',      'Triceps Brachii'),
  ('arms',      'forearm_flexors',      'Forearm Flexors'),
  ('arms',      'forearm_extensors',    'Forearm Extensors'),
  ('core',      'rectus_abdominis',     'Rectus Abdominis'),
  ('core',      'obliques',             'Obliques'),
  ('core',      'transversus_abdominis','Transversus Abdominis'),
  ('legs',      'gluteus_maximus',      'Gluteus Maximus'),
  ('legs',      'gluteus_medius',       'Gluteus Medius'),
  ('legs',      'quadriceps',           'Quadriceps'),
  ('legs',      'hamstrings',           'Hamstrings'),
  ('legs',      'adductors',            'Adductors'),
  ('legs',      'hip_flexors',          'Hip Flexors'),
  ('legs',      'calves',               'Calves')
) as m (group_code, code, name_en)
join public.muscle_groups g on g.code = m.group_code
on conflict (code) do nothing;
