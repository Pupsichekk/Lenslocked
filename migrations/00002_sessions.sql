-- +goose Up
-- +goose StatementBegin
create table sessions (
  id serial primary key,
  user_id int unique references users (id) on delete cascade,
  token_hash text unique not null
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table sessions;
-- +goose StatementEnd
