-- docker run --init -it --rm --platform linux/amd64 --name "app" --network faNetwork -v .:/app -w /app postgres:16-bookworm sh -c "PGPASSWORD=postgres psql -h faDb -U postgres -d fusionauth -f 2createMockData.sql"

BEGIN;

\set fusionauth_app_id 'e9fdb985-9173-4e01-9d73-ac2d60d1dc8e'

-- delete logins fusionauth creates when registering a user
DELETE FROM raw_logins WHERE applications_id = :'fusionauth_app_id';

-- randomize registration dates. a day from 2015 to 2025
UPDATE user_registrations SET insert_instant = (EXTRACT(EPOCH FROM ('2015-01-01'::DATE + (FLOOR(('2025-12-20'::DATE - '2015-01-01'::DATE) * random()))::INT)::TIMESTAMP) * 1000)::BIGINT;

-- set 5% users to unverified
UPDATE identities
SET verified = CASE
	WHEN random() < 0.05 THEN false
	ELSE true
END
WHERE identities.is_primary = true;

UPDATE identities
SET verified_reason = 5
WHERE identities.is_primary = true;

-- add login dates
WITH constants AS (
	SELECT
		(EXTRACT(EPOCH FROM '2025-12-30 00:00:00'::TIMESTAMP) * 1000)::BIGINT AS endOf2025Instant,
		(15 * 24 * 60 * 60 * 1000)::BIGINT AS millisecondsPer15Days,
		'127.0.0.1'::TEXT AS defaultIpAddress
),
loginDateLimitsPerUser AS (
	SELECT
		user_registrations.users_id,
		MIN(user_registrations.insert_instant) AS registrationInstant,
		FLOOR(((SELECT endOf2025Instant FROM constants) - MIN(user_registrations.insert_instant)) / (SELECT millisecondsPer15Days FROM constants))::INT AS maxLoginsLimit
	FROM
		user_registrations
	GROUP BY
		user_registrations.users_id
),
loginRowsToInsert AS (
	SELECT
		loginDateLimitsPerUser.users_id,
		loginDateLimitsPerUser.registrationInstant,
		generate_series(1, FLOOR(random() * (loginDateLimitsPerUser.maxLoginsLimit + 1))::INT) AS loginRowId
	FROM
		loginDateLimitsPerUser
)
INSERT INTO raw_logins (applications_id, instant, ip_address, identities_value, identities_type, users_id)
SELECT
	:'fusionauth_app_id',
	(l.registrationInstant + FLOOR(random() * ((SELECT endOf2025Instant FROM constants) - l.registrationInstant)))::BIGINT,
	(SELECT defaultIpAddress FROM constants),
	NULL,
	NULL,
	l.users_id
FROM
	loginRowsToInsert l;


-- delete logins for unverified users
delete from raw_logins
where users_id in (select users_id from identities WHERE is_primary=true and verified=false);

COMMIT;