package com.java.philly.jud.mininote.service;

import com.java.philly.jud.mininote.model.MiniNote;
import org.springframework.ui.Model;
import org.springframework.web.multipart.MultipartFile;

public interface IMiniNoteService {


     void getAllNotes(Model model);

     void uploadImage(MultipartFile file, String description, Model model);

     void saveMiniNote(String description, Model model);

     void deleteMiniNote(String note, Model model);
}
