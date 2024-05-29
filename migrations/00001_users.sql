-- +goose Up
-- +goose StatementBegin
create table users (
  id serial primary key,
  email text unique not null,
  password_hash text not null
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table users;
-- +goose StatementEnd
