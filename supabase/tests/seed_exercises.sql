-- Exercise seed assertions. Read-only; run with psql against a migrated DB.
do $$
declare
  n   int;
  bad int;
begin
  select count(*) into n from public.exercises;
  if n < 40 then
    raise exception 'ASSERT FAILED: expected >= 40 exercises, got %', n;
  end if;

  select count(*) into bad
  from public.exercises e
  where not exists (
          select 1 from public.exercise_muscles m
          where m.exercise_id = e.id and m.role = 'primary')
     or not exists (
          select 1 from public.exercise_equipment q
          where q.exercise_id = e.id);
  if bad > 0 then
    raise exception 'ASSERT FAILED: % exercises lack a primary muscle or equipment', bad;
  end if;
end $$;

select 'SEED CHECKS PASSED' as result;
