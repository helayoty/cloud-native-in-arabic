package com.java.philly.jud.mininote.repository;

import org.springframework.data.mongodb.repository.MongoRepository;
import com.java.philly.jud.mininote.model.MiniNote;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface MiniNoteRepository extends MongoRepository<MiniNote, String> {

    List<MiniNote> findAll();
}
