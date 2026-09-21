---
title: Notes on "Building Event-Driven Microservices"
subtitle: "Reading through the 2nd edition by Adam Bellemare"
slug: notes-on-event-driven-microservices
date: 2026-07-16
tags: [essays, books]
status: draft
reading_time_override: null
---

One of the first big things starting from this book was about Conway's law from Martin Melvin Conway. "Organizations which design systems are constrained to produce designs which are copies of the communication structures of these organizations."
This makes me think about work behind and our centralization versus decentralization both in software and in leadership. Often work mind will plan centrally but execute decentrally and that also lends itself well to microservices.We can do scoping and process requirements centrally, but then accept any arbitrary container that fulfills the requirements. And that works for us.

I learned about message passing architecture versus publish subscribe or durable architectures I like message passing as a term because it is it communicates the simplicity a bit better. Message passing services don't have to concern themselves with durability, guaranteed delivery, or things like that. WorkMind uses message passing for its conversation updates and its template updates as well. This works because the messages themselves do not carry state. They just carry a delta and the authoritative state is not a replay of events, but just the object in the database. And we can just use CRUD endpoints to synchronize the state. The events and the messages, they just communicate state drift.

The book recommends a structure of an event with three parts: record, key, and a value. The record is kind of like the header. It's got metadata about the event itself. The key is an identifier, and then the value is the payload. I think we have something similar at work mind, but it would be worth checking.

Talking about Lambda and Kappa architecture, Adam really goes hard on Lambda. He does not like that at all. And he brings up, well his reasons why is because it creates diverging truths. It also puts the entire consumer load on the producer service when the consumer wants to load store data. It kind of hits you twice because it has to do the stream and the store data. The biggest is that he says data is not written atomically and it's difficult to get high performance distributed transactions across multiple independent systems. The producer has to update one system first, usually the historic data, and then send an event. And keeping those two tasks aligned is difficult. However, if you just rely on your event stream, there's only one source of truth, but you do kind of have to continually aggregate things and hydrate the state.

So if I wanted to move work mind to a more Kappa architecture, while I would be able to get rid of a lot of the state tables for things like templates or conversations, and then I would replace those endpoints with microservices that just do aggregations of the events for that topic. I would still very much need the database for holding data from client-tether or systems of record. I think I would want event logs, of course, in Postgres. And it would probably be helpful to keep the snapshots, the high-dated state, probably be nice to keep it in the database, but it's being updated by its own dedicated microservice. I mean, for a work mind, how hard would it be to create an artifact from the event stream? It's not like we have thousands of events. It's from a few hundreds. So that could basically just be a single SQL query. 


