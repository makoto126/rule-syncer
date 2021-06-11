FROM alpine

ADD rule-syncer /usr/local/bin
CMD ["rule-syncer"]