package service

import (
	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type UserService struct {
	userRepo   *repository.UserRepo
	followRepo *repository.FollowRepo
	postRepo   *repository.PostRepo
}

func NewUserService() *UserService {
	return &UserService{
		userRepo:   repository.NewUserRepo(),
		followRepo: repository.NewFollowRepo(),
		postRepo:   repository.NewPostRepo(),
	}
}

// ProfileData holds all data needed for the user profile page.
type ProfileData struct {
	User        *model.User
	IsFollowing bool
	Following   int64
	Followers   int64
	PostCount   int64
	Posts       []model.Post
	TotalPosts  int64
}

func (s *UserService) GetProfile(profileUserID, currentUserID int64) (*ProfileData, error) {
	user, err := s.userRepo.FindByID(profileUserID)
	if err != nil || user == nil {
		return nil, err
	}

	isFollowing := false
	if currentUserID > 0 && currentUserID != profileUserID {
		isFollowing = s.followRepo.IsFollowing(currentUserID, profileUserID)
	}

	following, followers := s.userRepo.GetFollowCounts(profileUserID)
	postCount := s.postRepo.CountByUser(profileUserID)

	q := repository.FeedQuery{
		UserID:   &profileUserID,
		Status:   model.StatusApproved,
		Sort:     model.SortLatest,
		Page:     1,
		PageSize: 12,
	}
	posts, total, _ := s.postRepo.Feed(q)

	return &ProfileData{
		User:        user,
		IsFollowing: isFollowing,
		Following:   following,
		Followers:   followers,
		PostCount:   postCount,
		Posts:       posts,
		TotalPosts:  total,
	}, nil
}
